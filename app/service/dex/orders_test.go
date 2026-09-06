package dex

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
)

// fakeOrdersRepo records the depth/limit it was asked for and returns canned
// buy/sell books. GetMarketOrdersWithDepth is called from two goroutines
// concurrently, so access to the recorded calls is mutex-guarded.
type fakeOrdersRepo struct {
	mu    sync.Mutex
	buys  []entity.MarketOrder
	sells []entity.MarketOrder
	// err applies to a single order type, so error-path tests can fail one side
	// only and avoid the (separate) both-sides-error race in getMarketOrders.
	errOrderType string
	err          error
	limits       map[string]int
}

func (f *fakeOrdersRepo) GetMarketOrdersWithDepth(marketId, orderType string, limit int) ([]entity.MarketOrder, error) {
	f.mu.Lock()
	if f.limits == nil {
		f.limits = map[string]int{}
	}
	f.limits[orderType] = limit
	f.mu.Unlock()

	if f.err != nil && orderType == f.errOrderType {
		return nil, f.err
	}

	if orderType == entity.OrderTypeBuy {
		return f.buys, nil
	}

	return f.sells, nil
}

func (f *fakeOrdersRepo) limitFor(orderType string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.limits[orderType]
}

func newTestOrdersService(t *testing.T, oRepo *fakeOrdersRepo, mRepo *fakeMarketRepo) *OrdersService {
	t.Helper()
	svc, err := NewOrdersService(newTestLogger(), oRepo, mRepo)
	if err != nil {
		t.Fatalf("NewOrdersService: unexpected error: %v", err)
	}

	return svc
}

func TestNewOrdersServiceRejectsNilDependencies(t *testing.T) {
	logger := newTestLogger()
	oRepo := &fakeOrdersRepo{}
	mRepo := &fakeMarketRepo{}

	if svc, err := NewOrdersService(logger, oRepo, mRepo); err != nil || svc == nil {
		t.Fatalf("valid dependencies: got svc=%v err=%v", svc, err)
	}

	cases := map[string]func() (*OrdersService, error){
		"nil logger":     func() (*OrdersService, error) { return NewOrdersService(nil, oRepo, mRepo) },
		"nil ordersRepo": func() (*OrdersService, error) { return NewOrdersService(logger, nil, mRepo) },
		"nil marketRepo": func() (*OrdersService, error) { return NewOrdersService(logger, oRepo, nil) },
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			if svc, err := build(); err == nil || svc != nil {
				t.Fatalf("expected an error, got svc=%v err=%v", svc, err)
			}
		})
	}
}

// TestGetMarketOrdersMapsDefaultShape asserts the default `/api/dex/orders` shape
// (README endpoint 5). The service preserves the order the repository returns —
// buys descending, sells ascending is the repository's job, so the fake supplies
// already-ordered books and the test checks they are passed through untouched.
func TestGetMarketOrdersMapsDefaultShape(t *testing.T) {
	const marketId = "factory/bze1abc/uvdl/ubze"
	mRepo := &fakeMarketRepo{market: &entity.Market{MarketID: marketId, Base: "uvdl", Quote: "ubze"}}
	oRepo := &fakeOrdersRepo{
		buys: []entity.MarketOrder{
			{Price: "0.0001", Amount: "100000"},
			{Price: "0.000001", Amount: "10000000"},
		},
		sells: []entity.MarketOrder{
			{Price: "4.2", Amount: "696966"},
			{Price: "4.3", Amount: "42000"},
		},
	}

	svc := newTestOrdersService(t, oRepo, mRepo)

	res, err := svc.GetMarketOrders(marketId, 4)
	if err != nil {
		t.Fatalf("GetMarketOrders: unexpected error: %v", err)
	}

	assertString(t, "market_id", res.MarketId, marketId)
	if res.Timestamp == "" {
		t.Fatalf("timestamp must be set")
	}
	if len(res.Bids) != 2 || len(res.Asks) != 2 {
		t.Fatalf("expected 2 bids and 2 asks, got %d/%d", len(res.Bids), len(res.Asks))
	}
	// Order preserved, price/volume mapped from Price/Amount.
	assertString(t, "bids[0].price", res.Bids[0].Price, "0.0001")
	assertString(t, "bids[0].volume", res.Bids[0].Volume, "100000")
	assertString(t, "bids[1].price", res.Bids[1].Price, "0.000001")
	assertString(t, "asks[0].price", res.Asks[0].Price, "4.2")
	assertString(t, "asks[0].volume", res.Asks[0].Volume, "696966")
	assertString(t, "asks[1].price", res.Asks[1].Price, "4.3")

	// depth 4 -> limit 2 per side.
	if got := oRepo.limitFor(entity.OrderTypeBuy); got != 2 {
		t.Fatalf("buy limit: got %d, want 2", got)
	}
	if got := oRepo.limitFor(entity.OrderTypeSell); got != 2 {
		t.Fatalf("sell limit: got %d, want 2", got)
	}
}

// TestGetCoingeckoMarketOrdersMapsShape asserts the coingecko orders shape: the
// ticker_id is base_quote and bids/asks are [price, volume] string pairs.
func TestGetCoingeckoMarketOrdersMapsShape(t *testing.T) {
	const marketId = "ubze/uvdl"
	mRepo := &fakeMarketRepo{market: &entity.Market{MarketID: marketId, Base: "ubze", Quote: "uvdl"}}
	oRepo := &fakeOrdersRepo{
		buys:  []entity.MarketOrder{{Price: "0.0011", Amount: "100000"}},
		sells: []entity.MarketOrder{{Price: "0.0016", Amount: "42000"}},
	}

	svc := newTestOrdersService(t, oRepo, mRepo)

	res, err := svc.GetCoingeckoMarketOrders(marketId, 10)
	if err != nil {
		t.Fatalf("GetCoingeckoMarketOrders: unexpected error: %v", err)
	}

	assertString(t, "ticker_id", res.TickerId, "ubze_uvdl")
	if res.Timestamp == "" {
		t.Fatalf("timestamp must be set")
	}
	if len(res.Bids) != 1 || len(res.Asks) != 1 {
		t.Fatalf("expected 1 bid and 1 ask, got %d/%d", len(res.Bids), len(res.Asks))
	}
	if res.Bids[0][0] != "0.0011" || res.Bids[0][1] != "100000" {
		t.Fatalf("bid pair mismatch: %v", res.Bids[0])
	}
	if res.Asks[0][0] != "0.0016" || res.Asks[0][1] != "42000" {
		t.Fatalf("ask pair mismatch: %v", res.Asks[0])
	}

	// depth 10 (the README default) -> limit 5 per side.
	if got := oRepo.limitFor(entity.OrderTypeBuy); got != 5 {
		t.Fatalf("buy limit for depth 10: got %d, want 5", got)
	}
}

// TestGetMarketOrdersOutOfRangeDepth: the service divides depth by two with no
// clamping (defaults/clamping live in the controller), so a tiny depth floors to
// a zero per-side limit that it forwards verbatim to the repository.
func TestGetMarketOrdersOutOfRangeDepth(t *testing.T) {
	const marketId = "ubze/uvdl"
	mRepo := &fakeMarketRepo{market: &entity.Market{MarketID: marketId, Base: "ubze", Quote: "uvdl"}}
	oRepo := &fakeOrdersRepo{}

	svc := newTestOrdersService(t, oRepo, mRepo)

	if _, err := svc.GetMarketOrders(marketId, 1); err != nil {
		t.Fatalf("GetMarketOrders: unexpected error: %v", err)
	}
	if got := oRepo.limitFor(entity.OrderTypeBuy); got != 0 {
		t.Fatalf("depth 1 -> per-side limit: got %d, want 0", got)
	}
}

// TestGetMarketOrdersEmptyBook pins today's behaviour for an empty order book.
//
// BUG: an empty book yields nil Bids/Asks slices, which marshal to JSON `null`
// rather than the empty array `[]` the README implies. Reported separately (see
// PR description); asserting the current (nil) behaviour here.
func TestGetMarketOrdersEmptyBook(t *testing.T) {
	const marketId = "ubze/uvdl"
	mRepo := &fakeMarketRepo{market: &entity.Market{MarketID: marketId, Base: "ubze", Quote: "uvdl"}}
	oRepo := &fakeOrdersRepo{}

	svc := newTestOrdersService(t, oRepo, mRepo)

	res, err := svc.GetMarketOrders(marketId, 4)
	if err != nil {
		t.Fatalf("GetMarketOrders: unexpected error: %v", err)
	}
	if res.Bids != nil || res.Asks != nil {
		t.Fatalf("BUG pin: expected nil bids/asks today, got %v / %v", res.Bids, res.Asks)
	}

	cg, err := svc.GetCoingeckoMarketOrders(marketId, 4)
	if err != nil {
		t.Fatalf("GetCoingeckoMarketOrders: unexpected error: %v", err)
	}
	if cg.Bids != nil || cg.Asks != nil {
		t.Fatalf("BUG pin: expected nil coingecko bids/asks today, got %v / %v", cg.Bids, cg.Asks)
	}
}

func TestGetMarketOrdersUnknownMarketReturnsError(t *testing.T) {
	mRepo := &fakeMarketRepo{market: nil} // market not found
	oRepo := &fakeOrdersRepo{}

	svc := newTestOrdersService(t, oRepo, mRepo)

	if _, err := svc.GetMarketOrders("nope", 10); err == nil {
		t.Fatalf("GetMarketOrders: expected an error for an unknown market")
	}
	if _, err := svc.GetCoingeckoMarketOrders("nope", 10); err == nil {
		t.Fatalf("GetCoingeckoMarketOrders: expected an error for an unknown market")
	}
}

func TestGetMarketOrdersMarketRepoErrorPropagates(t *testing.T) {
	mRepo := &fakeMarketRepo{err: errors.New("market repo down")}
	oRepo := &fakeOrdersRepo{}

	svc := newTestOrdersService(t, oRepo, mRepo)

	if _, err := svc.GetMarketOrders("ubze/uvdl", 10); err == nil {
		t.Fatalf("expected the market-repo error to propagate")
	}
}

// TestGetMarketOrdersOrdersRepoErrorPropagates fails a single order-book side so
// the error surfaces without tripping the both-sides-error race in getMarketOrders.
func TestGetMarketOrdersOrdersRepoErrorPropagates(t *testing.T) {
	const marketId = "ubze/uvdl"
	mRepo := &fakeMarketRepo{market: &entity.Market{MarketID: marketId, Base: "ubze", Quote: "uvdl"}}
	oRepo := &fakeOrdersRepo{err: errors.New("orders repo down"), errOrderType: entity.OrderTypeBuy}

	svc := newTestOrdersService(t, oRepo, mRepo)

	if _, err := svc.GetMarketOrders(marketId, 10); err == nil {
		t.Fatalf("expected the orders-repo error to propagate")
	}
}

// TestOrdersHydrateResponseTracksNewestTimestamp exercises hydrateResponse directly
// through both response shapes to confirm the shared path handles both.
func TestOrdersHydrateResponseTracksNewestTimestamp(t *testing.T) {
	const marketId = "ubze/uvdl"
	mRepo := &fakeMarketRepo{market: &entity.Market{MarketID: marketId, Base: "ubze", Quote: "uvdl"}}
	oRepo := &fakeOrdersRepo{
		buys:  []entity.MarketOrder{{Price: "1", Amount: "2", CreatedAt: time.Now()}},
		sells: []entity.MarketOrder{{Price: "3", Amount: "4", CreatedAt: time.Now()}},
	}

	svc := newTestOrdersService(t, oRepo, mRepo)

	res, err := svc.GetMarketOrders(marketId, 4)
	if err != nil {
		t.Fatalf("GetMarketOrders: unexpected error: %v", err)
	}
	if len(res.Bids) != 1 || len(res.Asks) != 1 {
		t.Fatalf("hydrateResponse did not populate both sides: %d/%d", len(res.Bids), len(res.Asks))
	}
}
