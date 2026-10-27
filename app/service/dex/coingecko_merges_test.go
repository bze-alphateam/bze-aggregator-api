package dex

import (
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto/request"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/response"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
)

// The USDC.n → USDC.inj case: CoinGecko tracks the old market's ticker, the new
// market is folded into it.
const (
	oldUsdcMarket = "ubze/usdcn"
	newUsdcMarket = "ubze/usdcinj"
	otherMarket   = "ubze/uvdl"
)

func usdcMerges(t *testing.T) CoingeckoMerges {
	t.Helper()
	merges, err := NewCoingeckoMerges(map[string]string{newUsdcMarket: oldUsdcMarket})
	if err != nil {
		t.Fatalf("NewCoingeckoMerges: unexpected error: %v", err)
	}

	return merges
}

// fakeBooksRepo returns a book per market id.
type fakeBooksRepo struct {
	mu    sync.Mutex
	buys  map[string][]entity.MarketOrder
	sells map[string][]entity.MarketOrder
}

func (f *fakeBooksRepo) GetMarketOrdersWithDepth(marketId, orderType string, limit int) ([]entity.MarketOrder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if orderType == entity.OrderTypeBuy {
		return f.buys[marketId], nil
	}

	return f.sells[marketId], nil
}

// fakeMarketsHistoryRepo returns the trades of the requested market and
// records every request.
type fakeMarketsHistoryRepo struct {
	byMarket map[string][]entity.MarketHistory
	params   []request.HistoryParams
}

func (f *fakeMarketsHistoryRepo) GetHistoryBy(params request.HistoryParams) ([]entity.MarketHistory, error) {
	f.params = append(f.params, params)

	return f.byMarket[params.MarketId], nil
}

func (f *fakeMarketsHistoryRepo) GetAddressSwapHistory(address string) ([]entity.MarketHistory, error) {
	return nil, nil
}

func TestNewCoingeckoMergesRejectsInvalidMappings(t *testing.T) {
	cases := map[string]map[string]string{
		"folded into itself": {oldUsdcMarket: oldUsdcMarket},
		"reported is folded": {newUsdcMarket: oldUsdcMarket, oldUsdcMarket: otherMarket},
	}

	for name, folded := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewCoingeckoMerges(folded); err == nil {
				t.Fatalf("expected an error for %v", folded)
			}
		})
	}

	if merges, err := NewCoingeckoMerges(nil); err != nil || len(merges) != 0 {
		t.Fatalf("nil mapping: got merges=%v err=%v", merges, err)
	}
}

// usdcTickersDeps sets up the old market (quiet book, some volume), the new one
// (trading, with orders) and an unrelated market.
func usdcTickersDeps() (*fakeTickersMarketRepo, *fakeIntervalsRepo, *fakeTickersOrdersRepo, *fakePriceCalculator, *fakeLiquidityDataRepo, *fakeSupplyService) {
	mRepo, iRepo, oRepo, pc, lRepo, ss := validTickersDeps()
	mRepo.markets = []entity.MarketWithLastPrice{
		{Market: entity.Market{MarketID: oldUsdcMarket, Base: "ubze", Quote: "usdcn"}, LastPrice: sql.NullString{String: "0.00030", Valid: true}},
		{Market: entity.Market{MarketID: otherMarket, Base: "ubze", Quote: "uvdl"}, LastPrice: sql.NullString{String: "2", Valid: true}},
		{Market: entity.Market{MarketID: newUsdcMarket, Base: "ubze", Quote: "usdcinj"}, LastPrice: sql.NullString{String: "0.00032", Valid: true}},
	}
	iRepo.byMarket[oldUsdcMarket] = []entity.MarketHistoryInterval{
		{OpenPrice: "0.00031", HighestPrice: "0.00036", LowestPrice: "0.00029", BaseVolume: "100", QuoteVolume: "0.03"},
	}
	iRepo.byMarket[newUsdcMarket] = []entity.MarketHistoryInterval{
		{OpenPrice: "0.00031", HighestPrice: "0.00033", LowestPrice: "0.00027", BaseVolume: "50", QuoteVolume: "0.016"},
	}
	iRepo.byMarket[otherMarket] = []entity.MarketHistoryInterval{
		{OpenPrice: "2", HighestPrice: "2", LowestPrice: "2", BaseVolume: "7", QuoteVolume: "14"},
	}
	oRepo.highestBuy[oldUsdcMarket] = &entity.MarketOrder{Price: "0.00020"}
	oRepo.lowestSell[oldUsdcMarket] = &entity.MarketOrder{Price: "0.00060"}
	oRepo.highestBuy[newUsdcMarket] = &entity.MarketOrder{Price: "0.00031"}
	oRepo.lowestSell[newUsdcMarket] = &entity.MarketOrder{Price: "0.00033"}

	return mRepo, iRepo, oRepo, pc, lRepo, ss
}

func newMergingTickersService(t *testing.T, mRepo *fakeTickersMarketRepo, iRepo *fakeIntervalsRepo, oRepo *fakeTickersOrdersRepo, pc *fakePriceCalculator, lRepo *fakeLiquidityDataRepo, ss *fakeSupplyService) *Tickers {
	t.Helper()
	svc, err := NewTickersService(newTestLogger(), mRepo, iRepo, oRepo, pc, lRepo, ss, usdcMerges(t))
	if err != nil {
		t.Fatalf("NewTickersService: unexpected error: %v", err)
	}

	return svc
}

func tickersById(tickers []*response.CoingeckoTicker) map[string]*response.CoingeckoTicker {
	byId := make(map[string]*response.CoingeckoTicker, len(tickers))
	for _, ti := range tickers {
		byId[ti.TickerId] = ti
	}

	return byId
}

// TestGetCoingeckoTickersFoldsTheNewMarketIntoTheTrackedTicker asserts the old
// ticker id carries both markets' volume, the new market's last price and book
// top, and that the new ticker is not listed (no double counting).
func TestGetCoingeckoTickersFoldsTheNewMarketIntoTheTrackedTicker(t *testing.T) {
	mRepo, iRepo, oRepo, pc, lRepo, ss := usdcTickersDeps()
	svc := newMergingTickersService(t, mRepo, iRepo, oRepo, pc, lRepo, ss)

	tickers, err := svc.GetCoingeckoTickers()
	if err != nil {
		t.Fatalf("GetCoingeckoTickers: unexpected error: %v", err)
	}
	if len(tickers) != 2 {
		t.Fatalf("expected 2 tickers (old USDC + other), got %d", len(tickers))
	}

	byId := tickersById(tickers)
	if _, ok := byId["ubze_usdcinj"]; ok {
		t.Fatalf("the folded market must not be listed")
	}

	got, ok := byId["ubze_usdcn"]
	if !ok {
		t.Fatalf("the tracked ticker ubze_usdcn is missing")
	}
	assertString(t, "base_currency", got.Base, "ubze")
	assertString(t, "target_currency", got.Quote, "usdcn")
	assertString(t, "pool_id", got.MarketId, oldUsdcMarket)
	assertFloat64(t, "base_volume", got.BaseVolume, 150)
	assertFloat64(t, "target_volume", got.QuoteVolume, 0.046)
	assertFloat64(t, "high", got.High, 0.00036)
	assertFloat64(t, "low", got.Low, 0.00027)
	assertFloat64(t, "last_price", got.LastPrice, 0.00032)
	assertFloat64(t, "bid", got.Bid, 0.00031)
	assertFloat64(t, "ask", got.Ask, 0.00033)

	other := byId["ubze_uvdl"]
	assertFloat64(t, "other base_volume", other.BaseVolume, 7)
}

// TestGetCoingeckoTickersKeepsTheOldPriceAndBookWhileTheNewMarketIsQuiet covers
// the time before trading moves: no trades and no orders on the new market.
func TestGetCoingeckoTickersKeepsTheOldPriceAndBookWhileTheNewMarketIsQuiet(t *testing.T) {
	mRepo, iRepo, oRepo, pc, lRepo, ss := usdcTickersDeps()
	mRepo.markets[2].LastPrice = sql.NullString{}
	delete(iRepo.byMarket, newUsdcMarket)
	delete(oRepo.highestBuy, newUsdcMarket)
	delete(oRepo.lowestSell, newUsdcMarket)
	svc := newMergingTickersService(t, mRepo, iRepo, oRepo, pc, lRepo, ss)

	tickers, err := svc.GetCoingeckoTickers()
	if err != nil {
		t.Fatalf("GetCoingeckoTickers: unexpected error: %v", err)
	}

	got := tickersById(tickers)["ubze_usdcn"]
	assertFloat64(t, "base_volume", got.BaseVolume, 100)
	assertFloat64(t, "low", got.Low, 0.00029)
	assertFloat64(t, "last_price", got.LastPrice, 0.0003)
	assertFloat64(t, "bid", got.Bid, 0.0002)
	assertFloat64(t, "ask", got.Ask, 0.0006)
}

// TestGetCoingeckoTickersListsTheFoldedMarketWhenTheTrackedOneIsMissing keeps a
// folded market's own ticker when the market it folds into is not listed.
func TestGetCoingeckoTickersListsTheFoldedMarketWhenTheTrackedOneIsMissing(t *testing.T) {
	mRepo, iRepo, oRepo, pc, lRepo, ss := usdcTickersDeps()
	mRepo.markets = mRepo.markets[1:]
	svc := newMergingTickersService(t, mRepo, iRepo, oRepo, pc, lRepo, ss)

	tickers, err := svc.GetCoingeckoTickers()
	if err != nil {
		t.Fatalf("GetCoingeckoTickers: unexpected error: %v", err)
	}

	got, ok := tickersById(tickers)["ubze_usdcinj"]
	if !ok || len(tickers) != 2 {
		t.Fatalf("expected the new market's own ticker, got %d tickers", len(tickers))
	}
	assertFloat64(t, "base_volume", got.BaseVolume, 50)
}

// TestGetTickersKeepsMergedMarketsSeparate asserts the default format ignores
// the merges.
func TestGetTickersKeepsMergedMarketsSeparate(t *testing.T) {
	mRepo, iRepo, oRepo, pc, lRepo, ss := usdcTickersDeps()
	svc := newMergingTickersService(t, mRepo, iRepo, oRepo, pc, lRepo, ss)

	tickers, err := svc.GetTickers()
	if err != nil {
		t.Fatalf("GetTickers: unexpected error: %v", err)
	}
	if len(tickers) != 3 {
		t.Fatalf("expected 3 separate tickers, got %d", len(tickers))
	}

	for _, ti := range tickers {
		switch ti.MarketId {
		case oldUsdcMarket:
			assertFloat64(t, "old base_volume", ti.BaseVolume, 100)
		case newUsdcMarket:
			assertFloat64(t, "new base_volume", ti.BaseVolume, 50)
		}
	}
}

func newMergingOrdersService(t *testing.T, oRepo *fakeBooksRepo) *OrdersService {
	t.Helper()
	mRepo := &fakeMarketRepo{market: &entity.Market{MarketID: oldUsdcMarket, Base: "ubze", Quote: "usdcn"}}
	svc, err := NewOrdersService(newTestLogger(), oRepo, mRepo, usdcMerges(t))
	if err != nil {
		t.Fatalf("NewOrdersService: unexpected error: %v", err)
	}

	return svc
}

// TestGetCoingeckoMarketOrdersServesTheNewBookUnderTheTrackedTicker asserts the
// depth comes from the new market, without mixing in the old book.
func TestGetCoingeckoMarketOrdersServesTheNewBookUnderTheTrackedTicker(t *testing.T) {
	oRepo := &fakeBooksRepo{
		buys: map[string][]entity.MarketOrder{
			oldUsdcMarket: {{Price: "0.0002", Amount: "5"}},
			newUsdcMarket: {{Price: "0.00031", Amount: "10"}},
		},
		sells: map[string][]entity.MarketOrder{
			oldUsdcMarket: {{Price: "0.0006", Amount: "5"}},
			newUsdcMarket: {{Price: "0.00033", Amount: "20"}},
		},
	}
	svc := newMergingOrdersService(t, oRepo)

	got, err := svc.GetCoingeckoMarketOrders(oldUsdcMarket, 10)
	if err != nil {
		t.Fatalf("GetCoingeckoMarketOrders: unexpected error: %v", err)
	}

	assertString(t, "ticker_id", got.TickerId, "ubze_usdcn")
	if len(got.Bids) != 1 || len(got.Asks) != 1 {
		t.Fatalf("expected the new book only, got %d bids / %d asks", len(got.Bids), len(got.Asks))
	}
	assertString(t, "bid", got.Bids[0][0], "0.00031")
	assertString(t, "ask", got.Asks[0][0], "0.00033")
}

func TestGetCoingeckoMarketOrdersFallsBackToTheOldBookWhenTheNewOneIsEmpty(t *testing.T) {
	oRepo := &fakeBooksRepo{
		buys:  map[string][]entity.MarketOrder{oldUsdcMarket: {{Price: "0.0002", Amount: "5"}}},
		sells: map[string][]entity.MarketOrder{oldUsdcMarket: {{Price: "0.0006", Amount: "5"}}},
	}
	svc := newMergingOrdersService(t, oRepo)

	got, err := svc.GetCoingeckoMarketOrders(oldUsdcMarket, 10)
	if err != nil {
		t.Fatalf("GetCoingeckoMarketOrders: unexpected error: %v", err)
	}

	assertString(t, "bid", got.Bids[0][0], "0.0002")
	assertString(t, "ask", got.Asks[0][0], "0.0006")
}

func TestGetMarketOrdersKeepsMergedMarketsSeparate(t *testing.T) {
	oRepo := &fakeBooksRepo{
		buys:  map[string][]entity.MarketOrder{oldUsdcMarket: {{Price: "0.0002", Amount: "5"}}, newUsdcMarket: {{Price: "0.00031", Amount: "10"}}},
		sells: map[string][]entity.MarketOrder{},
	}
	svc := newMergingOrdersService(t, oRepo)

	got, err := svc.GetMarketOrders(oldUsdcMarket, 10)
	if err != nil {
		t.Fatalf("GetMarketOrders: unexpected error: %v", err)
	}

	assertString(t, "bid", got.Bids[0].Price, "0.0002")
}

func newMergingHistoryService(t *testing.T, repo *fakeMarketsHistoryRepo) *HistoryService {
	t.Helper()
	svc, err := NewHistoryService(newTestLogger(), repo, usdcMerges(t))
	if err != nil {
		t.Fatalf("NewHistoryService: unexpected error: %v", err)
	}

	return svc
}

// TestGetCoingeckoHistoryMergesTheFoldedMarketsTrades asserts both markets'
// trades come back newest first, cut to the limit, for the tracked ticker id.
func TestGetCoingeckoHistoryMergesTheFoldedMarketsTrades(t *testing.T) {
	at := func(minute int) time.Time { return time.Date(2026, 10, 27, 12, minute, 0, 0, time.UTC) }
	repo := &fakeMarketsHistoryRepo{byMarket: map[string][]entity.MarketHistory{
		oldUsdcMarket: {
			{ID: 3, OrderType: entity.OrderTypeBuy, ExecutedAt: at(30)},
			{ID: 1, OrderType: entity.OrderTypeSell, ExecutedAt: at(10)},
		},
		newUsdcMarket: {
			{ID: 4, OrderType: entity.OrderTypeSell, ExecutedAt: at(40)},
			{ID: 2, OrderType: entity.OrderTypeBuy, ExecutedAt: at(20)},
		},
	}}
	svc := newMergingHistoryService(t, repo)

	got, err := svc.GetCoingeckoHistory(&request.HistoryParams{TickerId: "ubze_usdcn", Limit: 3})
	if err != nil {
		t.Fatalf("GetCoingeckoHistory: unexpected error: %v", err)
	}

	if len(got.Buy) != 2 || len(got.Sell) != 1 {
		t.Fatalf("expected trades 4, 3, 2 (2 buys, 1 sell), got %d buys / %d sells", len(got.Buy), len(got.Sell))
	}
	if got.Sell[0].OrderId != 4 || got.Buy[0].OrderId != 3 || got.Buy[1].OrderId != 2 {
		t.Fatalf("unexpected trades: sells %+v buys %+v", got.Sell, got.Buy)
	}

	if len(repo.params) != 2 || repo.params[0].MarketId != oldUsdcMarket || repo.params[1].MarketId != newUsdcMarket {
		t.Fatalf("expected one query per market, got %+v", repo.params)
	}
}

// TestGetHistoryFiltersByTickerId is the regression test for ticker_id-only
// requests, which used to return every market's trades: the market id was not
// resolved from the ticker id before the query.
func TestGetHistoryFiltersByTickerId(t *testing.T) {
	repo := &fakeMarketsHistoryRepo{byMarket: map[string][]entity.MarketHistory{
		otherMarket: {{ID: 1, OrderType: entity.OrderTypeBuy}},
	}}
	svc := newMergingHistoryService(t, repo)

	got, err := svc.GetHistory(&request.HistoryParams{TickerId: "ubze_uvdl"})
	if err != nil {
		t.Fatalf("GetHistory: unexpected error: %v", err)
	}
	if len(got) != 1 || repo.params[0].MarketId != otherMarket {
		t.Fatalf("expected the query filtered by %s, got params %+v", otherMarket, repo.params)
	}

	if _, err = svc.GetCoingeckoHistory(&request.HistoryParams{TickerId: "ubze_uvdl"}); err != nil {
		t.Fatalf("GetCoingeckoHistory: unexpected error: %v", err)
	}
	if last := repo.params[len(repo.params)-1]; last.MarketId != otherMarket {
		t.Fatalf("coingecko: expected the query filtered by %s, got %+v", otherMarket, last)
	}
}

// TestGetHistoryKeepsMergedMarketsSeparate asserts the default format reads the
// requested market only.
func TestGetHistoryKeepsMergedMarketsSeparate(t *testing.T) {
	repo := &fakeMarketsHistoryRepo{byMarket: map[string][]entity.MarketHistory{
		oldUsdcMarket: {{ID: 1, OrderType: entity.OrderTypeBuy}},
		newUsdcMarket: {{ID: 2, OrderType: entity.OrderTypeBuy}},
	}}
	svc := newMergingHistoryService(t, repo)

	got, err := svc.GetHistory(&request.HistoryParams{MarketId: oldUsdcMarket})
	if err != nil {
		t.Fatalf("GetHistory: unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].OrderId != 1 {
		t.Fatalf("expected the old market's trade only, got %+v", got)
	}
}
