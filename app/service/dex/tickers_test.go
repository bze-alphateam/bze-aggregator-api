package dex

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto/response"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
)

// --- fakes -------------------------------------------------------------------

type fakeTickersMarketRepo struct {
	markets []entity.MarketWithLastPrice
	err     error
}

func (f *fakeTickersMarketRepo) GetMarketsWithLastExecuted(hours int) ([]entity.MarketWithLastPrice, error) {
	return f.markets, f.err
}

type fakeIntervalsRepo struct {
	byMarket map[string][]entity.MarketHistoryInterval
	err      error
}

func (f *fakeIntervalsRepo) GetIntervalsByExecutedAt(marketId string, executedAt time.Time, length int) ([]entity.MarketHistoryInterval, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.byMarket[marketId], nil
}

type fakeTickersOrdersRepo struct {
	highestBuy    map[string]*entity.MarketOrder
	lowestSell    map[string]*entity.MarketOrder
	highestBuyErr error
	lowestSellErr error
}

func (f *fakeTickersOrdersRepo) GetHighestBuy(marketId string) (*entity.MarketOrder, error) {
	if f.highestBuyErr != nil {
		return nil, f.highestBuyErr
	}

	return f.highestBuy[marketId], nil
}

func (f *fakeTickersOrdersRepo) GetLowestSell(marketId string) (*entity.MarketOrder, error) {
	if f.lowestSellErr != nil {
		return nil, f.lowestSellErr
	}

	return f.lowestSell[marketId], nil
}

type fakePriceCalculator struct {
	prices map[string]float64
	err    error
}

func (f *fakePriceCalculator) CalculateInternalPrice(denom string) (float64, error) {
	if f.err != nil {
		return 0, f.err
	}

	return f.prices[denom], nil
}

type fakeLiquidityDataRepo struct {
	data  map[string]*entity.MarketLiquidityData
	err   error
	calls []string
}

func (f *fakeLiquidityDataRepo) GetLiquidityDataByMarketId(marketId string) (*entity.MarketLiquidityData, error) {
	f.calls = append(f.calls, marketId)
	if f.err != nil {
		return nil, f.err
	}

	return f.data[marketId], nil
}

type fakeSupplyService struct {
	supply map[string]string
	err    error
}

func (f *fakeSupplyService) GetUTotalSupply(denom string) (string, error) {
	if f.err != nil {
		return "", f.err
	}

	return f.supply[denom], nil
}

// --- helpers -----------------------------------------------------------------

func validTickersDeps() (*fakeTickersMarketRepo, *fakeIntervalsRepo, *fakeTickersOrdersRepo, *fakePriceCalculator, *fakeLiquidityDataRepo, *fakeSupplyService) {
	return &fakeTickersMarketRepo{},
		&fakeIntervalsRepo{byMarket: map[string][]entity.MarketHistoryInterval{}},
		&fakeTickersOrdersRepo{highestBuy: map[string]*entity.MarketOrder{}, lowestSell: map[string]*entity.MarketOrder{}},
		&fakePriceCalculator{prices: map[string]float64{}},
		&fakeLiquidityDataRepo{data: map[string]*entity.MarketLiquidityData{}},
		&fakeSupplyService{supply: map[string]string{}}
}

func newTestTickersService(t *testing.T, mRepo *fakeTickersMarketRepo, iRepo *fakeIntervalsRepo, oRepo *fakeTickersOrdersRepo, pc *fakePriceCalculator, lRepo *fakeLiquidityDataRepo, ss *fakeSupplyService) *Tickers {
	t.Helper()
	svc, err := NewTickersService(newTestLogger(), mRepo, iRepo, oRepo, pc, lRepo, ss)
	if err != nil {
		t.Fatalf("NewTickersService: unexpected error: %v", err)
	}

	return svc
}

// --- constructor -------------------------------------------------------------

func TestNewTickersServiceRejectsNilDependencies(t *testing.T) {
	mRepo, iRepo, oRepo, pc, lRepo, ss := validTickersDeps()
	logger := newTestLogger()

	// All valid -> no error.
	if svc, err := NewTickersService(logger, mRepo, iRepo, oRepo, pc, lRepo, ss); err != nil || svc == nil {
		t.Fatalf("valid dependencies: got svc=%v err=%v", svc, err)
	}

	cases := map[string]func() (*Tickers, error){
		"nil logger":        func() (*Tickers, error) { return NewTickersService(nil, mRepo, iRepo, oRepo, pc, lRepo, ss) },
		"nil marketRepo":    func() (*Tickers, error) { return NewTickersService(logger, nil, iRepo, oRepo, pc, lRepo, ss) },
		"nil intervalsRepo": func() (*Tickers, error) { return NewTickersService(logger, mRepo, nil, oRepo, pc, lRepo, ss) },
		"nil ordersRepo":    func() (*Tickers, error) { return NewTickersService(logger, mRepo, iRepo, nil, pc, lRepo, ss) },
		"nil priceCalc":     func() (*Tickers, error) { return NewTickersService(logger, mRepo, iRepo, oRepo, nil, lRepo, ss) },
		"nil liquidityRepo": func() (*Tickers, error) { return NewTickersService(logger, mRepo, iRepo, oRepo, pc, nil, ss) },
		"nil supplyService": func() (*Tickers, error) { return NewTickersService(logger, mRepo, iRepo, oRepo, pc, lRepo, nil) },
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			svc, err := build()
			if err == nil || svc != nil {
				t.Fatalf("expected an error, got svc=%v err=%v", svc, err)
			}
		})
	}
}

// --- GetTickers (default shape) ----------------------------------------------

// TestGetTickersMapsEveryFieldForASingleMarket asserts the default `/api/dex/tickers`
// shape field by field against the README endpoint-4 sample contract.
func TestGetTickersMapsEveryFieldForASingleMarket(t *testing.T) {
	const marketId = "ubze/uvdl"
	mRepo, iRepo, oRepo, pc, lRepo, ss := validTickersDeps()
	mRepo.markets = []entity.MarketWithLastPrice{
		{
			Market:    entity.Market{MarketID: marketId, Base: "ubze", Quote: "uvdl"},
			LastPrice: sql.NullString{String: "110", Valid: true},
		},
	}
	oRepo.highestBuy[marketId] = &entity.MarketOrder{Price: "0.0011"}
	oRepo.lowestSell[marketId] = &entity.MarketOrder{Price: "0.0016"}
	iRepo.byMarket[marketId] = []entity.MarketHistoryInterval{
		{OpenPrice: "100", HighestPrice: "120", LowestPrice: "110", BaseVolume: "10", QuoteVolume: "1000"},
		{OpenPrice: "105", HighestPrice: "130", LowestPrice: "90", BaseVolume: "5", QuoteVolume: "500"},
	}

	svc := newTestTickersService(t, mRepo, iRepo, oRepo, pc, lRepo, ss)

	tickers, err := svc.GetTickers()
	if err != nil {
		t.Fatalf("GetTickers: unexpected error: %v", err)
	}
	if len(tickers) != 1 {
		t.Fatalf("expected 1 ticker, got %d", len(tickers))
	}

	got := tickers[0]
	assertString(t, "base", got.Base, "ubze")
	assertString(t, "quote", got.Quote, "uvdl")
	assertString(t, "market_id", got.MarketId, marketId)
	assertFloat64(t, "last_price", got.LastPrice, 110)
	assertFloat64(t, "bid", got.Bid, 0.0011)
	assertFloat64(t, "ask", got.Ask, 0.0016)
	// open comes from intervals[0].OpenPrice; high is the max HighestPrice; low is the
	// min LowestPrice; volumes are the sum across intervals.
	assertFloat64(t, "open_price", got.OpenPrice, 100)
	assertFloat64(t, "high", got.High, 130)
	assertFloat64(t, "low", got.Low, 90)
	assertFloat64(t, "base_volume", got.BaseVolume, 15)
	assertFloat64(t, "quote_volume", got.QuoteVolume, 1500)
	// change = ((last-open)/open)*100 rounded to 2 decimals = ((110-100)/100)*100 = 10.
	if got.Change != 10 {
		t.Fatalf("change: got %v, want 10", got.Change)
	}
}

// TestGetTickersMarketWithNoHistoryNoOrders documents the shape of a brand-new
// market: every numeric field is zero, only base/quote/market_id are populated.
func TestGetTickersMarketWithNoHistoryNoOrders(t *testing.T) {
	const marketId = "unew/uvdl"
	mRepo, iRepo, oRepo, pc, lRepo, ss := validTickersDeps()
	mRepo.markets = []entity.MarketWithLastPrice{
		{Market: entity.Market{MarketID: marketId, Base: "unew", Quote: "uvdl"}}, // LastPrice invalid
	}

	svc := newTestTickersService(t, mRepo, iRepo, oRepo, pc, lRepo, ss)

	tickers, err := svc.GetTickers()
	if err != nil {
		t.Fatalf("GetTickers: unexpected error: %v", err)
	}
	if len(tickers) != 1 {
		t.Fatalf("expected 1 ticker, got %d", len(tickers))
	}

	got := tickers[0]
	assertString(t, "base", got.Base, "unew")
	assertString(t, "quote", got.Quote, "uvdl")
	assertString(t, "market_id", got.MarketId, marketId)
	assertFloat64(t, "last_price", got.LastPrice, 0)
	assertFloat64(t, "bid", got.Bid, 0)
	assertFloat64(t, "ask", got.Ask, 0)
	assertFloat64(t, "open_price", got.OpenPrice, 0)
	assertFloat64(t, "high", got.High, 0)
	assertFloat64(t, "low", got.Low, 0)
	assertFloat64(t, "base_volume", got.BaseVolume, 0)
	assertFloat64(t, "quote_volume", got.QuoteVolume, 0)
	if got.Change != 0 {
		t.Fatalf("change: got %v, want 0", got.Change)
	}
}

// TestGetTickersMarketsRepoErrorPropagates: an error listing the markets aborts
// the whole response.
func TestGetTickersMarketsRepoErrorPropagates(t *testing.T) {
	mRepo, iRepo, oRepo, pc, lRepo, ss := validTickersDeps()
	mRepo.err = errors.New("boom")

	svc := newTestTickersService(t, mRepo, iRepo, oRepo, pc, lRepo, ss)

	tickers, err := svc.GetTickers()
	if err == nil {
		t.Fatalf("expected an error, got nil")
	}
	if tickers != nil {
		t.Fatalf("expected nil tickers on markets-repo error, got %v", tickers)
	}
}

// TestGetTickersReturnsErrorButStillAppendsWhenAMarketFails pins today's behaviour
// when building a single ticker fails: buildTicker returns the error, GetTickers
// surfaces it AND still appends the partially-built ticker to the slice.
//
// BUG: GetTickers/GetCoingeckoTickers share the outer `err` variable and write
// `gErr` from every worker goroutine without holding the mutex, so with two or
// more markets those writes race (go test -race would flag it on the normal
// production path, where many markets exist). These tests use a single market on
// purpose to stay race-free; the race is reported separately (see PR description).
func TestGetTickersReturnsErrorButStillAppendsWhenAMarketFails(t *testing.T) {
	const marketId = "ubze/uvdl"
	mRepo, iRepo, oRepo, pc, lRepo, ss := validTickersDeps()
	mRepo.markets = []entity.MarketWithLastPrice{
		{Market: entity.Market{MarketID: marketId, Base: "ubze", Quote: "uvdl"}},
	}
	oRepo.highestBuyErr = errors.New("orders repo down")

	svc := newTestTickersService(t, mRepo, iRepo, oRepo, pc, lRepo, ss)

	tickers, err := svc.GetTickers()
	if err == nil {
		t.Fatalf("expected an error when a market fails, got nil")
	}
	if len(tickers) != 1 {
		t.Fatalf("expected the partial ticker to still be appended, got %d tickers", len(tickers))
	}
}

// --- GetCoingeckoTickers (coingecko shape + liquidity) -----------------------

// TestGetCoingeckoTickersMapsEveryFieldForAnLpMarket asserts the coingecko shape
// field by field (README endpoint-4 coingecko sample) and, because the market id
// contains "_", exercises the LP liquidity_in_usd calculation and the ticker_id /
// pool_id "lp_" composition.
func TestGetCoingeckoTickersMapsEveryFieldForAnLpMarket(t *testing.T) {
	const (
		marketId = "ubze_uvdl"
		lpDenom  = "ulpubzeuvdl"
	)
	mRepo, iRepo, oRepo, pc, lRepo, ss := validTickersDeps()
	mRepo.markets = []entity.MarketWithLastPrice{
		{
			Market:    entity.Market{MarketID: marketId, Base: "ubze", Quote: "uvdl"},
			LastPrice: sql.NullString{String: "110", Valid: true},
		},
	}
	oRepo.highestBuy[marketId] = &entity.MarketOrder{Price: "0.0011"}
	oRepo.lowestSell[marketId] = &entity.MarketOrder{Price: "0.0016"}
	iRepo.byMarket[marketId] = []entity.MarketHistoryInterval{
		{OpenPrice: "100", HighestPrice: "120", LowestPrice: "110", BaseVolume: "10", QuoteVolume: "1000"},
		{OpenPrice: "105", HighestPrice: "130", LowestPrice: "90", BaseVolume: "5", QuoteVolume: "500"},
	}
	lRepo.data[marketId] = &entity.MarketLiquidityData{LpDenom: lpDenom}
	pc.prices[lpDenom] = 2.0
	ss.supply[lpDenom] = "3000000000000000" // 3e15 micro units -> 3000 whole (12 decimals)

	svc := newTestTickersService(t, mRepo, iRepo, oRepo, pc, lRepo, ss)

	tickers, err := svc.GetCoingeckoTickers()
	if err != nil {
		t.Fatalf("GetCoingeckoTickers: unexpected error: %v", err)
	}
	if len(tickers) != 1 {
		t.Fatalf("expected 1 ticker, got %d", len(tickers))
	}

	got := tickers[0]
	// ticker_id and pool_id are both prefixed with "lp_" when the market id is a pool.
	assertString(t, "ticker_id", got.TickerId, "lp_ubze_uvdl")
	assertString(t, "pool_id", got.MarketId, "lp_ubze_uvdl")
	assertString(t, "base_currency", got.Base, "ubze")
	assertString(t, "target_currency", got.Quote, "uvdl")
	assertFloat64(t, "last_price", got.LastPrice, 110)
	assertFloat64(t, "bid", got.Bid, 0.0011)
	assertFloat64(t, "ask", got.Ask, 0.0016)
	assertFloat64(t, "high", got.High, 130)
	assertFloat64(t, "low", got.Low, 90)
	assertFloat64(t, "base_volume", got.BaseVolume, 15)
	assertFloat64(t, "target_volume", got.QuoteVolume, 1500)
	// liquidity = price(2.0) * wholeSupply(3000) = 6000.
	assertFloat64(t, "liquidity_in_usd", got.LiquidityInUsd, 6000)
}

// --- calculateAndSetLiquidity edge cases -------------------------------------

func TestCalculateAndSetLiquiditySkipsNonPoolMarket(t *testing.T) {
	mRepo, iRepo, oRepo, pc, lRepo, ss := validTickersDeps()
	svc := newTestTickersService(t, mRepo, iRepo, oRepo, pc, lRepo, ss)

	tk := &response.CoingeckoTicker{}
	svc.calculateAndSetLiquidity("ubze/uvdl", tk) // no underscore -> not a pool

	if len(lRepo.calls) != 0 {
		t.Fatalf("liquidity repo must not be queried for a non-pool market, calls=%v", lRepo.calls)
	}
	assertFloat64(t, "liquidity_in_usd", tk.LiquidityInUsd, 0)
}

func TestCalculateAndSetLiquiditySupplyErrorLeavesZero(t *testing.T) {
	const (
		marketId = "ubze_uvdl"
		lpDenom  = "ulpubzeuvdl"
	)
	mRepo, iRepo, oRepo, pc, lRepo, ss := validTickersDeps()
	lRepo.data[marketId] = &entity.MarketLiquidityData{LpDenom: lpDenom}
	pc.prices[lpDenom] = 2.0
	ss.err = errors.New("supply service down")

	svc := newTestTickersService(t, mRepo, iRepo, oRepo, pc, lRepo, ss)

	tk := &response.CoingeckoTicker{}
	svc.calculateAndSetLiquidity(marketId, tk)

	assertFloat64(t, "liquidity_in_usd", tk.LiquidityInUsd, 0)
}

func TestCalculateAndSetLiquidityUnparsableSupplyLeavesZero(t *testing.T) {
	const (
		marketId = "ubze_uvdl"
		lpDenom  = "ulpubzeuvdl"
	)
	mRepo, iRepo, oRepo, pc, lRepo, ss := validTickersDeps()
	lRepo.data[marketId] = &entity.MarketLiquidityData{LpDenom: lpDenom}
	pc.prices[lpDenom] = 2.0
	ss.supply[lpDenom] = "not-a-number"

	svc := newTestTickersService(t, mRepo, iRepo, oRepo, pc, lRepo, ss)

	tk := &response.CoingeckoTicker{}
	svc.calculateAndSetLiquidity(marketId, tk)

	assertFloat64(t, "liquidity_in_usd", tk.LiquidityInUsd, 0)
}
