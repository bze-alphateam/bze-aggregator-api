package dex

import (
	"errors"
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto/request"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
)

type fakeHistoryRepo struct {
	history    []entity.MarketHistory
	historyErr error
	lastParams request.HistoryParams

	swap    []entity.MarketHistory
	swapErr error
}

func (f *fakeHistoryRepo) GetHistoryBy(params request.HistoryParams) ([]entity.MarketHistory, error) {
	f.lastParams = params

	return f.history, f.historyErr
}

func (f *fakeHistoryRepo) GetAddressSwapHistory(address string) ([]entity.MarketHistory, error) {
	return f.swap, f.swapErr
}

func newTestHistoryService(t *testing.T, repo *fakeHistoryRepo) *HistoryService {
	t.Helper()
	svc, err := NewHistoryService(newTestLogger(), repo)
	if err != nil {
		t.Fatalf("NewHistoryService: unexpected error: %v", err)
	}

	return svc
}

func TestNewHistoryServiceRejectsNilDependencies(t *testing.T) {
	logger := newTestLogger()
	repo := &fakeHistoryRepo{}

	if svc, err := NewHistoryService(logger, repo); err != nil || svc == nil {
		t.Fatalf("valid dependencies: got svc=%v err=%v", svc, err)
	}

	cases := map[string]func() (*HistoryService, error){
		"nil logger":      func() (*HistoryService, error) { return NewHistoryService(nil, repo) },
		"nil historyRepo": func() (*HistoryService, error) { return NewHistoryService(logger, nil) },
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			if svc, err := build(); err == nil || svc != nil {
				t.Fatalf("expected an error, got svc=%v err=%v", svc, err)
			}
		})
	}
}

// TestGetHistoryMapsDefaultShape asserts the default `/api/dex/history` shape field
// by field (README endpoint 6), including the millisecond `executed_at` string.
// Filtering (limit/type/address/start/end) is applied by the repository SQL, so
// the service test only asserts that the params are forwarded unchanged.
func TestGetHistoryMapsDefaultShape(t *testing.T) {
	executed := time.UnixMilli(1731016441000)
	repo := &fakeHistoryRepo{history: []entity.MarketHistory{
		{
			ID:          436157,
			OrderType:   entity.OrderTypeSell,
			Amount:      "12",
			Price:       "0.00156",
			QuoteAmount: "0.01872",
			ExecutedAt:  executed,
			Maker:       "bze1maker",
			Taker:       "bze1taker",
		},
	}}
	svc := newTestHistoryService(t, repo)

	params := &request.HistoryParams{MarketId: "ubze/uvdl", OrderType: "sell", Limit: 10, StartTime: 1, EndTime: 2, Address: "bze1x"}
	got, err := svc.GetHistory(params)
	if err != nil {
		t.Fatalf("GetHistory: unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(got))
	}

	tr := got[0]
	if tr.OrderId != 436157 {
		t.Fatalf("order_id: got %d, want 436157", tr.OrderId)
	}
	assertString(t, "price", tr.Price, "0.00156")
	assertString(t, "base_volume", tr.BaseVolume, "12")
	assertString(t, "quote_volume", tr.QuoteVolume, "0.01872")
	assertString(t, "executed_at", tr.ExecutedAt, "1731016441000")
	assertString(t, "order_type", tr.OrderType, "sell")
	assertString(t, "maker", tr.Maker, "bze1maker")
	assertString(t, "taker", tr.Taker, "bze1taker")

	// Params are forwarded to the repository verbatim.
	if repo.lastParams != *params {
		t.Fatalf("params forwarded to repo differ: got %+v, want %+v", repo.lastParams, *params)
	}
}

// TestGetHistoryEmptyResult pins today's behaviour: an empty history yields a nil
// slice.
//
// BUG: nil marshals to JSON `null` instead of the empty array `[]` that the README
// implies and that GetAddressSwapHistory already returns (it uses make([], 0)).
// Reported separately (see PR description); asserting the current (nil) behaviour.
func TestGetHistoryEmptyResult(t *testing.T) {
	repo := &fakeHistoryRepo{history: nil}
	svc := newTestHistoryService(t, repo)

	got, err := svc.GetHistory(&request.HistoryParams{MarketId: "ubze/uvdl"})
	if err != nil {
		t.Fatalf("GetHistory: unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("BUG pin: expected nil slice today, got %v", got)
	}
}

func TestGetHistoryRepoErrorPropagates(t *testing.T) {
	repo := &fakeHistoryRepo{historyErr: errors.New("history repo down")}
	svc := newTestHistoryService(t, repo)

	if _, err := svc.GetHistory(&request.HistoryParams{MarketId: "ubze/uvdl"}); err == nil {
		t.Fatalf("expected the history-repo error to propagate")
	}
}

// TestGetCoingeckoHistorySplitsByType asserts the coingecko history shape and the
// split of trades into the buy/sell buckets.
func TestGetCoingeckoHistorySplitsByType(t *testing.T) {
	executed := time.UnixMilli(1731016441000)
	repo := &fakeHistoryRepo{history: []entity.MarketHistory{
		{ID: 1, OrderType: entity.OrderTypeBuy, Amount: "10", Price: "0.00148", QuoteAmount: "0.0148", ExecutedAt: executed},
		{ID: 2, OrderType: entity.OrderTypeSell, Amount: "12", Price: "0.00156", QuoteAmount: "0.01872", ExecutedAt: executed},
	}}
	svc := newTestHistoryService(t, repo)

	got, err := svc.GetCoingeckoHistory(&request.HistoryParams{MarketId: "ubze/uvdl"})
	if err != nil {
		t.Fatalf("GetCoingeckoHistory: unexpected error: %v", err)
	}
	if len(got.Buy) != 1 || len(got.Sell) != 1 {
		t.Fatalf("expected 1 buy and 1 sell, got %d/%d", len(got.Buy), len(got.Sell))
	}

	buy := got.Buy[0]
	if buy.OrderId != 1 {
		t.Fatalf("trade_id: got %d, want 1", buy.OrderId)
	}
	assertString(t, "price", buy.Price, "0.00148")
	assertString(t, "base_volume", buy.BaseVolume, "10")
	assertString(t, "target_volume", buy.QuoteVolume, "0.0148")
	assertString(t, "trade_timestamp", buy.ExecutedAt, "1731016441000")
	assertString(t, "type", buy.OrderType, "buy")
	assertString(t, "sell type", got.Sell[0].OrderType, "sell")
}

func TestGetCoingeckoHistoryEmptyResult(t *testing.T) {
	repo := &fakeHistoryRepo{history: nil}
	svc := newTestHistoryService(t, repo)

	got, err := svc.GetCoingeckoHistory(&request.HistoryParams{MarketId: "ubze/uvdl"})
	if err != nil {
		t.Fatalf("GetCoingeckoHistory: unexpected error: %v", err)
	}
	if got.Buy != nil || got.Sell != nil {
		t.Fatalf("expected nil buy/sell buckets on empty history, got %v / %v", got.Buy, got.Sell)
	}
}

func TestGetCoingeckoHistoryRepoErrorPropagates(t *testing.T) {
	repo := &fakeHistoryRepo{historyErr: errors.New("history repo down")}
	svc := newTestHistoryService(t, repo)

	if _, err := svc.GetCoingeckoHistory(&request.HistoryParams{MarketId: "ubze/uvdl"}); err == nil {
		t.Fatalf("expected the history-repo error to propagate")
	}
}

// TestGetAddressSwapHistoryMapsPoolDenoms asserts the swap-history mapping,
// including the pool_id -> base/quote decomposition, and that a row whose market
// id is not a valid pool id is skipped rather than failing the whole response.
func TestGetAddressSwapHistoryMapsPoolDenoms(t *testing.T) {
	executed := time.UnixMilli(1731016441000)
	repo := &fakeHistoryRepo{swap: []entity.MarketHistory{
		{ID: 1, MarketID: "ubze_uvdl", OrderType: entity.OrderTypeBuy, Amount: "10", Price: "0.1", QuoteAmount: "1", ExecutedAt: executed, Maker: "m", Taker: "t"},
		{ID: 2, MarketID: "not-a-pool", OrderType: entity.OrderTypeSell, Amount: "5", Price: "0.2", QuoteAmount: "1", ExecutedAt: executed},
	}}
	svc := newTestHistoryService(t, repo)

	got, err := svc.GetAddressSwapHistory("bze1addr")
	if err != nil {
		t.Fatalf("GetAddressSwapHistory: unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected the non-pool row to be skipped, got %d rows", len(got))
	}

	tr := got[0]
	if tr.OrderId != 1 {
		t.Fatalf("order_id: got %d, want 1", tr.OrderId)
	}
	assertString(t, "pool_id", tr.PoolId, "ubze_uvdl")
	assertString(t, "base", tr.Base, "ubze")
	assertString(t, "quote", tr.Quote, "uvdl")
	assertString(t, "executed_at", tr.ExecutedAt, "1731016441000")
}

// TestGetAddressSwapHistoryEmptyResult confirms the empty case returns a non-nil
// empty slice (this method already uses make([], 0)).
func TestGetAddressSwapHistoryEmptyResult(t *testing.T) {
	repo := &fakeHistoryRepo{swap: nil}
	svc := newTestHistoryService(t, repo)

	got, err := svc.GetAddressSwapHistory("bze1addr")
	if err != nil {
		t.Fatalf("GetAddressSwapHistory: unexpected error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected a non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(got))
	}
}

func TestGetAddressSwapHistoryRepoErrorPropagates(t *testing.T) {
	repo := &fakeHistoryRepo{swapErr: errors.New("swap repo down")}
	svc := newTestHistoryService(t, repo)

	if _, err := svc.GetAddressSwapHistory("bze1addr"); err == nil {
		t.Fatalf("expected the swap-repo error to propagate")
	}
}
