package compare_test

import (
	"context"
	"net/url"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/service/compare"
)

func endpoint(t *testing.T, name string) compare.Endpoint {
	t.Helper()

	eps, err := compare.Lookup(name)
	if err != nil || len(eps) != 1 {
		t.Fatalf("lookup %s: %v", name, err)
	}

	return eps[0]
}

// compared normalises (and reconciles) both answers, as a round does.
func compared(ep compare.Endpoint, req compare.Request, agg, scan string) []compare.Difference {
	na, ns := ep.Normalise(req, decodeJSON(agg)), ep.Normalise(req, decodeJSON(scan))
	if ep.Reconcile != nil {
		na, ns = ep.Reconcile(req, na, ns)
	}

	return compare.Diff(na, ns, ep.Tolerances)
}

func same(t *testing.T, ep compare.Endpoint, req compare.Request, agg, scan string) {
	t.Helper()

	if d := compared(ep, req, agg, scan); len(d) != 0 {
		t.Fatalf("expected a match, got %+v", d)
	}
}

func differs(t *testing.T, ep compare.Endpoint, req compare.Request, agg, scan string) {
	t.Helper()

	if d := compared(ep, req, agg, scan); len(d) == 0 {
		t.Fatal("expected a difference")
	}
}

func TestLookup(t *testing.T) {
	all, err := compare.Lookup("all")
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, ep := range all {
		names = append(names, ep.Name)
	}

	if !slices.Equal(names, []string{"orders", "swaps", "history", "intervals", "tickers"}) {
		t.Fatalf("unexpected registry %v", names)
	}

	if _, err := compare.Lookup("prices"); err == nil {
		t.Fatal("prices is not compared")
	}
}

func TestNormaliseOrders(t *testing.T) {
	ep := endpoint(t, "orders")
	req := compare.Request{Path: "/api/dex/orders", Query: url.Values{"market_id": {"ubze/uvdl"}}}

	same(t, ep, req,
		`{"market_id":"ubze/uvdl","timestamp":"2026-10-29 12:00:00","bids":null,"asks":[{"price":"1","volume":"2"}]}`,
		`{"market_id":"ubze/uvdl","timestamp":"1793275200","bids":[],"asks":[{"price":"1","volume":"2"}]}`)

	differs(t, ep, req,
		`{"market_id":"ubze/uvdl","timestamp":"1","bids":[],"asks":[{"price":"1","volume":"2"}]}`,
		`{"market_id":"ubze/uvdl","timestamp":"1","bids":[],"asks":[{"price":"1","volume":"3"}]}`)
}

func TestNormaliseSwaps(t *testing.T) {
	ep := endpoint(t, "swaps")
	req := compare.Request{Path: "/api/dex/swaps", Query: url.Values{"address": {"bze1a"}}}

	same(t, ep, req,
		`[{"order_id":7,"price":"2","base_volume":"1","executed_at":"1700000001000","taker":"bze1a"},
		  {"order_id":8,"price":"1","base_volume":"1","executed_at":"1700000001000","taker":"bze1a"}]`,
		`[{"order_id":1700000001000002,"price":"1","base_volume":"1","executed_at":"1700000001456","taker":"bze1a"},
		  {"order_id":1700000001000001,"price":"2","base_volume":"1","executed_at":"1700000001123","taker":"bze1a"}]`)

	differs(t, ep, req,
		`[{"order_id":7,"price":"2","base_volume":"1","executed_at":"1700000001000","taker":"bze1a"}]`,
		`[{"order_id":7,"price":"2","base_volume":"1","executed_at":"1700000002000","taker":"bze1a"}]`)
}

func historyTrade(id int, seconds int64, price string) map[string]any {
	return map[string]any{
		"order_id": id, "price": price, "base_volume": "1", "quote_volume": price,
		"executed_at": formatInt(seconds*1000 + int64(id)), "order_type": "buy", "maker": "bze1m", "taker": "bze1t",
	}
}

func formatInt(n int64) string {
	return strconv.FormatInt(n, 10)
}

func TestNormaliseHistorySameSecondOrder(t *testing.T) {
	ep := endpoint(t, "history")
	req := compare.Request{Path: "/api/dex/history", Query: url.Values{"market_id": {"ubze/uvdl"}, "limit": {"50"}}}

	agg := []any{historyTrade(3, 102, "3"), historyTrade(2, 101, "1"), historyTrade(1, 101, "2")}
	scan := []any{historyTrade(9, 102, "3"), historyTrade(8, 101, "2"), historyTrade(7, 101, "1")}
	same(t, ep, req, mustJSON(t, agg), mustJSON(t, scan))
}

func TestNormaliseHistoryFullPageCut(t *testing.T) {
	ep := endpoint(t, "history")
	req := compare.Request{Path: "/api/dex/history", Query: url.Values{"market_id": {"ubze/uvdl"}, "limit": {"3"}}}

	// the limit cuts second 101 after two trades on one side, after one on the other
	agg := []any{historyTrade(4, 102, "1"), historyTrade(3, 101, "1"), historyTrade(2, 101, "2")}
	scan := []any{historyTrade(9, 102, "1"), historyTrade(8, 101, "3"), historyTrade(7, 101, "4")}
	same(t, ep, req, mustJSON(t, agg), mustJSON(t, scan))

	// the newer seconds still have to match
	scan = []any{historyTrade(9, 102, "9"), historyTrade(8, 101, "3"), historyTrade(7, 101, "4")}
	differs(t, ep, req, mustJSON(t, agg), mustJSON(t, scan))

	// not a full page: nothing is dropped
	notFull := compare.Request{Path: "/api/dex/history", Query: url.Values{"market_id": {"ubze/uvdl"}, "limit": {"50"}}}
	scan = []any{historyTrade(9, 102, "1"), historyTrade(8, 101, "3"), historyTrade(7, 101, "4")}
	differs(t, ep, notFull, mustJSON(t, agg), mustJSON(t, scan))
}

func TestNormaliseHistoryScanSuperset(t *testing.T) {
	ep := endpoint(t, "history")
	req := compare.Request{Path: "/api/dex/history", Query: url.Values{"market_id": {"ubze/uvdl"}, "limit": {"50"}}}

	// the aggregator lost one trade of second 101 at a sync page boundary
	agg := []any{historyTrade(3, 102, "1"), historyTrade(2, 101, "1"), historyTrade(1, 100, "1")}
	scan := []any{historyTrade(9, 102, "1"), historyTrade(8, 101, "5"), historyTrade(7, 101, "1"), historyTrade(6, 100, "1")}
	same(t, ep, req, mustJSON(t, agg), mustJSON(t, scan))

	// not in a second the aggregator has
	scan = []any{historyTrade(9, 102, "1"), historyTrade(8, 101, "1"), historyTrade(7, 100, "1"), historyTrade(6, 99, "1")}
	differs(t, ep, req, mustJSON(t, agg), mustJSON(t, scan))

	// the aggregator has a trade Scan misses
	scan = []any{historyTrade(9, 102, "1"), historyTrade(8, 101, "5"), historyTrade(7, 100, "1")}
	differs(t, ep, req, mustJSON(t, agg), mustJSON(t, scan))

	// on full pages the extra row ends Scan's page a second earlier
	full := compare.Request{Path: "/api/dex/history", Query: url.Values{"market_id": {"ubze/uvdl"}, "limit": {"3"}}}
	agg = []any{historyTrade(3, 103, "1"), historyTrade(2, 102, "1"), historyTrade(1, 101, "1")}
	scan = []any{historyTrade(9, 103, "1"), historyTrade(8, 103, "5"), historyTrade(7, 102, "1")}
	same(t, ep, full, mustJSON(t, agg), mustJSON(t, scan))
}

// inDoubledWindow is a time inside the v8.0.x doubled-event window.
var inDoubledWindow = time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC).Unix()

func TestNormaliseHistoryDoubledEvents(t *testing.T) {
	ep := endpoint(t, "history")
	req := compare.Request{Path: "/api/dex/history", Query: url.Values{"address": {"bze1t"}}}
	w := inDoubledWindow

	// a v8.0.x doubled event is listed twice on the aggregator only
	agg := []any{historyTrade(2, w+1, "1"), historyTrade(1, w+1, "1"), historyTrade(0, w, "2")}
	scan := []any{historyTrade(5, w+1, "1"), historyTrade(4, w, "2")}
	same(t, ep, req, mustJSON(t, agg), mustJSON(t, scan))
}

func TestNormaliseHistoryIdenticalTradesOutsideWindow(t *testing.T) {
	ep := endpoint(t, "history")
	req := compare.Request{Path: "/api/dex/history", Query: url.Values{"address": {"bze1t"}}}
	now := newFakeClock().Now().Unix()

	// two equal resting orders filled by one taker in one block: two real
	// trades the aggregator lists, Scan losing one is a divergence
	agg := []any{historyTrade(2, now, "1"), historyTrade(1, now, "1")}
	scan := []any{historyTrade(5, now, "1")}
	differs(t, ep, req, mustJSON(t, agg), mustJSON(t, scan))
	same(t, ep, req, mustJSON(t, agg), mustJSON(t, agg))
}

func TestSwapsUnderscoreDenoms(t *testing.T) {
	ep := endpoint(t, "swaps")
	req := compare.Request{Path: "/api/dex/swaps", Query: url.Values{"address": {"bze1a"}}}

	same(t, ep, req,
		`[{"order_id":7,"price":"2","base_volume":"1","executed_at":"1700000001000","taker":"bze1a","base":"ubze","quote":"uvdl"}]`,
		`[{"order_id":9,"price":"3","base_volume":"1","executed_at":"1700000002000","taker":"bze1a","base":"factory/x/a_b","quote":"ubze"},
		  {"order_id":8,"price":"2","base_volume":"1","executed_at":"1700000001000","taker":"bze1a","base":"ubze","quote":"uvdl"}]`)
}

func TestNormaliseHistoryCoingecko(t *testing.T) {
	ep := endpoint(t, "history")
	req := compare.Request{Path: "/api/dex/history", Query: url.Values{
		"format": {"coingecko"}, "ticker_id": {"ubze_uvdl"}, "limit": {"3"},
	}}

	cg := func(id int, seconds int64, price string) map[string]any {
		return map[string]any{
			"trade_id": id, "price": price, "base_volume": "1", "target_volume": price,
			"trade_timestamp": formatInt(seconds*1000 + int64(id%1000)), "type": "buy",
		}
	}

	// a full page of 3 across both sides: the oldest second (100) is dropped on both
	agg := map[string]any{
		"buy":  []any{cg(3, 102, "1"), cg(1, 100, "7")},
		"sell": []any{cg(2, 101, "2")},
	}
	scan := map[string]any{
		"buy":  []any{cg(13, 102, "1")},
		"sell": []any{cg(12, 101, "2"), cg(11, 100, "9")},
	}
	same(t, ep, req, mustJSON(t, agg), mustJSON(t, scan))

	// a side is left out on both when it has no trades
	same(t, ep, req, `{}`, `{}`)
	differs(t, ep, req, `{"buy":[]}`, `{}`)
}

func TestNormaliseIntervalsTimeZones(t *testing.T) {
	ep := endpoint(t, "intervals")
	req := compare.Request{Path: "/api/dex/intervals", Query: url.Values{"market_id": {"ubze/uvdl"}, "minutes": {"60"}}}

	candle := func(start, end, close string) map[string]any {
		return map[string]any{
			"market_id": "ubze/uvdl", "minutes": 60, "start_at": start, "end_at": end,
			"lowest_price": "1", "open_price": "1", "average_price": "1", "highest_price": close,
			"close_price": close, "base_volume": "5", "quote_volume": "5",
		}
	}
	zero := func(start, end string) map[string]any {
		return map[string]any{
			"market_id": "ubze/uvdl", "minutes": 60, "start_at": start, "end_at": end,
			"lowest_price": "0", "open_price": "0", "average_price": "0", "highest_price": "0",
			"close_price": "0", "base_volume": "0", "quote_volume": "0",
		}
	}

	agg := []any{
		zero("2026-10-29T12:00:00+02:00", "2026-10-29T12:59:59+02:00"),
		candle("2026-10-29T13:00:00+02:00", "2026-10-29T13:59:59+02:00", "2"),
		zero("2026-10-29T14:00:00+02:00", "2026-10-29T14:59:59+02:00"),
	}
	scan := []any{
		candle("2026-10-29T11:00:00Z", "2026-10-29T11:59:59Z", "2"),
		zero("2026-10-29T12:00:00Z", "2026-10-29T12:59:59Z"),
	}
	same(t, ep, req, mustJSON(t, agg), mustJSON(t, scan))

	// a zero candle after the first traded one is kept
	differs(t, ep, req, mustJSON(t, agg), mustJSON(t, scan[:1]))

	// another instant differs
	differs(t, ep, req, mustJSON(t, agg[1:2]),
		mustJSON(t, []any{candle("2026-10-29T13:00:00Z", "2026-10-29T13:59:59Z", "2")}))
}

func TestNormaliseIntervalsTradingView(t *testing.T) {
	ep := endpoint(t, "intervals")
	req := compare.Request{Path: "/api/dex/intervals", Query: url.Values{"format": {"tv"}}}

	same(t, ep, req,
		`[{"time":1,"low":0,"open":0,"high":0,"close":0,"value":0},{"time":2,"low":1,"open":1,"high":2,"close":2,"value":3}]`,
		`[{"time":2,"low":1,"open":1,"high":2,"close":2,"value":3}]`)
}

func TestNormaliseTickers(t *testing.T) {
	ep := endpoint(t, "tickers")

	def := compare.Request{Path: "/api/dex/tickers", Query: url.Values{}}
	same(t, ep, def,
		`[{"market_id":"b/q","last_price":1},{"market_id":"a/q","last_price":2}]`,
		`[{"market_id":"a/q","last_price":2},{"market_id":"b/q","last_price":1}]`)

	cg := compare.Request{Path: "/api/dex/tickers", Query: url.Values{"format": {"coingecko"}}}
	same(t, ep, cg,
		`[{"ticker_id":"lp_b_q","liquidity_in_usd":1000.0001},{"ticker_id":"a_q","last_price":2}]`,
		`[{"ticker_id":"a_q","last_price":2},{"ticker_id":"lp_b_q","liquidity_in_usd":1000.0002}]`)
	differs(t, ep, cg,
		`[{"ticker_id":"lp_b_q","liquidity_in_usd":1000}]`,
		`[{"ticker_id":"lp_b_q","liquidity_in_usd":1002}]`)

	// pools priced through BZE differ by about 0.01 % (BFE-304)
	same(t, ep, cg,
		`[{"ticker_id":"lp_b_q","liquidity_in_usd":1000}]`,
		`[{"ticker_id":"lp_b_q","liquidity_in_usd":1000.1}]`)
}

func requests(t *testing.T, name string, src fakeSource) []string {
	t.Helper()

	reqs, err := endpoint(t, name).Sample(context.Background(), src, newFakeClock().Now())
	if err != nil {
		t.Fatal(err)
	}

	var out []string
	for _, r := range reqs {
		out = append(out, r.Path+"?"+r.Params())
	}

	return out
}

const tickersFixture = `[
	{"base":"ubze","quote":"uvdl","market_id":"ubze/uvdl","base_volume":10},
	{"base":"ibc/AB","quote":"ubze","market_id":"ibc/AB_ubze","base_volume":0},
	{"base":"ubze","quote":"uatom","market_id":"ubze/uatom","base_volume":3}
]`

func TestSampleOrders(t *testing.T) {
	got := requests(t, "orders", fakeSource{"/api/dex/tickers": tickersFixture})
	want := []string{
		"/api/dex/orders?depth=0&market_id=ubze%2Fuvdl",
		"/api/dex/orders?depth=10&market_id=ubze%2Fuvdl",
		"/api/dex/orders?depth=0&format=coingecko&ticker_id=ubze_uvdl",
		"/api/dex/orders?depth=0&market_id=ubze%2Fuatom",
		"/api/dex/orders?depth=10&market_id=ubze%2Fuatom",
		"/api/dex/orders?depth=0&format=coingecko&ticker_id=ubze_uatom",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestSampleSwaps(t *testing.T) {
	got := requests(t, "swaps", fakeSource{
		"/api/dex/swaps?address=": `[{"taker":"bze1a"},{"taker":"bze1b"},{"taker":"bze1a"},{"taker":""}]`,
	})

	if !slices.Equal(got, []string{"/api/dex/swaps?address=bze1a", "/api/dex/swaps?address=bze1b"}) {
		t.Fatalf("got %v", got)
	}
}

func TestSampleHistory(t *testing.T) {
	now := newFakeClock().Now()
	recent := formatInt(now.Add(-time.Hour).UnixMilli())
	old := formatInt(now.Add(-48 * time.Hour).UnixMilli())

	src := fakeSource{
		"/api/dex/tickers":                  tickersFixture,
		"/api/dex/tickers?format=coingecko": `[{"ticker_id":"ubze_uvdl"},{"ticker_id":"lp_ibc/AB_ubze"}]`,
		// uvdl: one trade in the last day; uatom: two, so it is the busiest
		"/api/dex/history?limit=50&market_id=ubze%2Fuvdl": `[{"executed_at":"` + recent + `","taker":"bze1x"},{"executed_at":"` + old + `","taker":"bze1y"}]`,
		"/api/dex/history?limit=50&market_id=ubze%2Fuatom": `[
			{"executed_at":"` + recent + `","taker":"bze1a"},
			{"executed_at":"` + recent + `","taker":"bze1b"},
			{"executed_at":"` + old + `","taker":"bze1a"}]`,
	}

	got := requests(t, "history", src)
	want := []string{
		"/api/dex/history?limit=50&market_id=ubze%2Fuvdl",
		"/api/dex/history?limit=50&market_id=ibc%2FAB_ubze",
		"/api/dex/history?limit=50&market_id=ubze%2Fuatom",
		"/api/dex/history?format=coingecko&limit=50&ticker_id=ubze_uvdl",
		"/api/dex/history?format=coingecko&limit=50&ticker_id=lp_ibc%2FAB_ubze",
		"/api/dex/history?address=bze1a&limit=100",
		"/api/dex/history?address=bze1a&limit=100&market_id=ubze%2Fuatom",
		"/api/dex/history?address=bze1b&limit=100",
		"/api/dex/history?address=bze1b&limit=100&market_id=ubze%2Fuatom",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestSampleIntervals(t *testing.T) {
	got := requests(t, "intervals", fakeSource{"/api/dex/tickers": `[{"market_id":"ibc/AB_ubze"}]`})
	want := []string{
		"/api/dex/intervals?limit=100&market_id=ibc%2FAB_ubze&minutes=5",
		"/api/dex/intervals?limit=100&market_id=ibc%2FAB_ubze&minutes=15",
		"/api/dex/intervals?limit=100&market_id=ibc%2FAB_ubze&minutes=60",
		"/api/dex/intervals?limit=100&market_id=ibc%2FAB_ubze&minutes=240",
		"/api/dex/intervals?limit=30&market_id=ibc%2FAB_ubze&minutes=1440",
		"/api/dex/intervals?format=tv&limit=100&market_id=ibc%2FAB_ubze&minutes=60",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestSampleTickers(t *testing.T) {
	got := requests(t, "tickers", fakeSource{})
	if !slices.Equal(got, []string{"/api/dex/tickers?", "/api/dex/tickers?format=coingecko"}) {
		t.Fatalf("got %v", got)
	}
}

func TestSampleFailsWithoutTickers(t *testing.T) {
	if _, err := endpoint(t, "orders").Sample(context.Background(), fakeSource{}, time.Now()); err == nil {
		t.Fatal("expected an error")
	}
}
