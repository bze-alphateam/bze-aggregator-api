package compare

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	pathTickers   = "/api/dex/tickers"
	pathOrders    = "/api/dex/orders"
	pathSwaps     = "/api/dex/swaps"
	pathHistory   = "/api/dex/history"
	pathIntervals = "/api/dex/intervals"

	formatCoingecko = "coingecko"

	// the aggregator's /api/dex/swaps answers an address's newest 100 swaps
	swapsLimit = 100
	// the aggregator's /api/dex/history limit: 500 by default, 1000 at most
	historyDefaultLimit = 500
	historyMaxLimit     = 1000

	// takers sampled from the busiest market's history
	historyTakers = 10
)

// Request is one GET compared on both hosts: the same path and query are
// appended to the aggregator's and Scan's base URL.
type Request struct {
	Path  string
	Query url.Values
}

// Params is the request's query string, as printed in a difference line.
func (r Request) Params() string {
	return r.Query.Encode()
}

// Source reads the aggregator while a round builds its sample set. Answers
// are decoded with UseNumber; a non-2xx answer is an error.
type Source interface {
	Get(ctx context.Context, path string, query url.Values) (any, error)
}

// Endpoint is a registry entry: how a round samples the endpoint's requests
// and how both sides' answers are normalised before they are compared.
type Endpoint struct {
	Name string
	// Sample lists the requests compared this round. now is the round's start.
	Sample func(ctx context.Context, src Source, now time.Time) ([]Request, error)
	// Normalise rewrites a decoded 2xx answer; it may change v in place.
	Normalise func(req Request, v any) any
	// Tolerances overrides the relative number tolerance per object key.
	Tolerances map[string]float64
}

// Endpoints is the registry, in the order `all` runs them.
func Endpoints() []Endpoint {
	return []Endpoint{
		{Name: "orders", Sample: sampleOrders, Normalise: normaliseOrders},
		{Name: "swaps", Sample: sampleSwaps, Normalise: normaliseSwaps},
		{Name: "history", Sample: sampleHistory, Normalise: normaliseHistory},
		{Name: "intervals", Sample: sampleIntervals, Normalise: normaliseIntervals},
		{
			Name:      "tickers",
			Sample:    sampleTickers,
			Normalise: normaliseTickers,
			// USD prices are cached for 60 s on both sides (BFE-304)
			Tolerances: map[string]float64{"liquidity_in_usd": 1e-6},
		},
	}
}

// Lookup returns the endpoints a name selects: one by its name, every one
// for "all".
func Lookup(name string) ([]Endpoint, error) {
	all := Endpoints()
	if name == "all" {
		return all, nil
	}

	for _, ep := range all {
		if ep.Name == name {
			return []Endpoint{ep}, nil
		}
	}

	return nil, fmt.Errorf("unknown endpoint %q, expected one of %s or all", name, strings.Join(Names(), ", "))
}

// Names lists the registry's endpoint names.
func Names() []string {
	var names []string
	for _, ep := range Endpoints() {
		names = append(names, ep.Name)
	}

	return names
}

// ---- sample sets ----

type market struct {
	id    string
	base  string
	quote string
	// traded in the last 24 hours (a base volume above zero)
	traded bool
}

// isPool is the aggregator's own test for a liquidity pool id (base_quote):
// order book market ids are base/quote, and IBC denoms put a "/" in pool ids
// too, so a "/" alone cannot tell them apart.
func isPool(id string) bool {
	return strings.Contains(id, "_")
}

func tickerMarkets(ctx context.Context, src Source) ([]market, error) {
	v, err := src.Get(ctx, pathTickers, nil)
	if err != nil {
		return nil, err
	}

	var markets []market
	for _, t := range objects(v) {
		id := str(t, "market_id")
		if id == "" {
			continue
		}

		markets = append(markets, market{
			id:     id,
			base:   str(t, "base"),
			quote:  str(t, "quote"),
			traded: number(t["base_volume"]) > 0,
		})
	}

	return markets, nil
}

func sampleOrders(ctx context.Context, src Source, _ time.Time) ([]Request, error) {
	markets, err := tickerMarkets(ctx, src)
	if err != nil {
		return nil, err
	}

	var reqs []Request
	for _, m := range markets {
		if isPool(m.id) {
			continue
		}

		reqs = append(reqs,
			Request{Path: pathOrders, Query: url.Values{"market_id": {m.id}, "depth": {"0"}}},
			Request{Path: pathOrders, Query: url.Values{"market_id": {m.id}, "depth": {"10"}}},
			Request{Path: pathOrders, Query: url.Values{
				"format":    {formatCoingecko},
				"ticker_id": {m.base + "_" + m.quote},
				"depth":     {"0"},
			}},
		)
	}

	return reqs, nil
}

func sampleSwaps(ctx context.Context, src Source, _ time.Time) ([]Request, error) {
	// an empty address answers everyone's newest 100 swaps on the aggregator;
	// Scan answers 400 to it, so it only samples and is never compared
	v, err := src.Get(ctx, pathSwaps, url.Values{"address": {""}})
	if err != nil {
		return nil, err
	}

	var reqs []Request
	for _, taker := range distinct(objects(v), "taker", 0) {
		reqs = append(reqs, Request{Path: pathSwaps, Query: url.Values{"address": {taker}}})
	}

	return reqs, nil
}

func sampleHistory(ctx context.Context, src Source, now time.Time) ([]Request, error) {
	markets, err := tickerMarkets(ctx, src)
	if err != nil {
		return nil, err
	}

	var reqs []Request
	for _, m := range markets {
		reqs = append(reqs, Request{Path: pathHistory, Query: url.Values{"market_id": {m.id}, "limit": {"50"}}})
	}

	cg, err := src.Get(ctx, pathTickers, url.Values{"format": {formatCoingecko}})
	if err != nil {
		return nil, err
	}

	for _, t := range objects(cg) {
		if id := str(t, "ticker_id"); id != "" {
			reqs = append(reqs, Request{Path: pathHistory, Query: url.Values{
				"format":    {formatCoingecko},
				"ticker_id": {id},
				"limit":     {"50"},
			}})
		}
	}

	busiest, takers, err := busiestMarket(ctx, src, markets, now)
	if err != nil {
		return nil, err
	}

	for _, a := range takers {
		reqs = append(reqs,
			Request{Path: pathHistory, Query: url.Values{"address": {a}, "limit": {"100"}}},
			Request{Path: pathHistory, Query: url.Values{"address": {a}, "market_id": {busiest}, "limit": {"100"}}},
		)
	}

	return reqs, nil
}

// busiestMarket is the market or pool with the most trades in the last 24
// hours among its newest 50 (the first one listed on a tie), with up to 10 of
// its recent takers, newest first. Only markets that traded in the last 24
// hours are read.
func busiestMarket(ctx context.Context, src Source, markets []market, now time.Time) (string, []string, error) {
	since := now.Add(-24 * time.Hour).UnixMilli()
	best, bestCount := "", 0
	var bestTrades []map[string]any

	for _, m := range markets {
		if !m.traded {
			continue
		}

		v, err := src.Get(ctx, pathHistory, url.Values{"market_id": {m.id}, "limit": {"50"}})
		if err != nil {
			return "", nil, err
		}

		trades := objects(v)
		count := 0
		for _, t := range trades {
			if ms, err := strconv.ParseInt(str(t, "executed_at"), 10, 64); err == nil && ms >= since {
				count++
			}
		}

		if count > bestCount {
			best, bestCount, bestTrades = m.id, count, trades
		}
	}

	return best, distinct(bestTrades, "taker", historyTakers), nil
}

func sampleIntervals(ctx context.Context, src Source, _ time.Time) ([]Request, error) {
	markets, err := tickerMarkets(ctx, src)
	if err != nil {
		return nil, err
	}

	var reqs []Request
	for _, m := range markets {
		for _, minutes := range []string{"5", "15", "60", "240"} {
			reqs = append(reqs, Request{Path: pathIntervals, Query: url.Values{
				"market_id": {m.id}, "minutes": {minutes}, "limit": {"100"},
			}})
		}

		reqs = append(reqs,
			// only 30 days: older daily candles differ because of the
			// v8.0.x doubled events (BFE-303)
			Request{Path: pathIntervals, Query: url.Values{"market_id": {m.id}, "minutes": {"1440"}, "limit": {"30"}}},
			Request{Path: pathIntervals, Query: url.Values{
				"market_id": {m.id}, "minutes": {"60"}, "limit": {"100"}, "format": {"tv"},
			}},
		)
	}

	return reqs, nil
}

func sampleTickers(context.Context, Source, time.Time) ([]Request, error) {
	return []Request{
		{Path: pathTickers, Query: url.Values{}},
		{Path: pathTickers, Query: url.Values{"format": {formatCoingecko}}},
	}, nil
}

// ---- normalisers ----

func normaliseOrders(_ Request, v any) any {
	o, ok := v.(map[string]any)
	if !ok {
		return v
	}

	// the answer's own time on each side (BFE-300)
	delete(o, "timestamp")

	// an empty side is [] on Scan and null on the aggregator (BFE-300)
	for _, side := range []string{"bids", "asks"} {
		if s, ok := o[side]; ok && s == nil {
			o[side] = []any{}
		}
	}

	return o
}

// tradeFields names a trade list's id and time keys.
type tradeFields struct {
	id   string
	time string
}

var (
	defaultTrade   = tradeFields{id: "order_id", time: "executed_at"}
	coingeckoTrade = tradeFields{id: "trade_id", time: "trade_timestamp"}
)

func normaliseSwaps(_ Request, v any) any {
	list, ok := v.([]any)
	if !ok {
		return v
	}

	return normaliseTrades([][]any{list}, defaultTrade, swapsLimit)[0]
}

func normaliseHistory(req Request, v any) any {
	limit := queryLimit(req, historyDefaultLimit, historyMaxLimit)

	if req.Query.Get("format") != formatCoingecko {
		list, ok := v.([]any)
		if !ok {
			return v
		}

		return normaliseTrades([][]any{list}, defaultTrade, limit)[0]
	}

	o, ok := v.(map[string]any)
	if !ok {
		return v
	}

	// the limit cuts the trades before they are split by side, so both sides
	// make up one page
	sides := []string{"buy", "sell"}
	lists := make([][]any, len(sides))
	for i, side := range sides {
		lists[i], _ = o[side].([]any)
	}

	lists = normaliseTrades(lists, coingeckoTrade, limit)
	for i, side := range sides {
		if _, ok := o[side]; ok {
			o[side] = lists[i]
		}
	}

	return o
}

// normaliseTrades applies the trade lists' allowed differences (BFE-301,
// BFE-302). lists are the parts of one page of at most limit trades.
func normaliseTrades(lists [][]any, f tradeFields, limit int) [][]any {
	total := 0
	for _, l := range lists {
		total += len(l)
	}

	full := limit > 0 && total >= limit

	out := make([][]any, len(lists))
	for i, l := range lists {
		for _, e := range l {
			t, ok := e.(map[string]any)
			if !ok {
				continue
			}

			// Scan's ids are stable trade ids, the aggregator's its row ids
			delete(t, f.id)
			// Scan has the block time in ms, the aggregator whole seconds
			if s, ok := truncateToSeconds(t[f.time]); ok {
				t[f.time] = s
			}
		}

		// trades within the same second can come in another order
		sortTrades(l, f.time)
		// v8.0.x doubled events are counted once on Scan, twice on the
		// aggregator
		out[i] = dedupe(l)
	}

	if !full {
		return out
	}

	// on a full page the limit can cut the oldest second in two different
	// places, so the trades of that second are dropped
	oldest := int64(-1)
	for _, l := range out {
		if len(l) == 0 {
			continue
		}

		if s := tradeSeconds(l[len(l)-1], f.time); oldest < 0 || s < oldest {
			oldest = s
		}
	}

	for i, l := range out {
		for len(l) > 0 && tradeSeconds(l[len(l)-1], f.time) == oldest {
			l = l[:len(l)-1]
		}

		out[i] = l
	}

	return out
}

func normaliseIntervals(req Request, v any) any {
	list, ok := v.([]any)
	if !ok {
		return v
	}

	zeroFields := []string{
		"lowest_price", "open_price", "average_price", "highest_price", "close_price", "base_volume", "quote_volume",
	}
	if req.Query.Get("format") == "tv" {
		zeroFields = []string{"low", "open", "high", "close", "value"}
	}

	for _, e := range list {
		c, ok := e.(map[string]any)
		if !ok {
			continue
		}

		// the aggregator prints the server's local time zone, Scan UTC
		for _, key := range []string{"start_at", "end_at"} {
			if s, ok := c[key].(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
					c[key] = t.UTC().Format(time.RFC3339Nano)
				}
			}
		}
	}

	// the pools' floors differ (Scan: the first swap, the aggregator: the
	// start of that day), so leading zero candles are dropped (BFE-303)
	for len(list) > 0 && isZeroCandle(list[0], zeroFields) {
		list = list[1:]
	}

	return list
}

func normaliseTickers(req Request, v any) any {
	list, ok := v.([]any)
	if !ok {
		return v
	}

	key := "market_id"
	if req.Query.Get("format") == formatCoingecko {
		key = "ticker_id"
	}

	// the two sides list the tickers in different orders (BFE-304)
	slices.SortStableFunc(list, func(a, b any) int {
		return strings.Compare(str(asObject(a), key), str(asObject(b), key))
	})

	return list
}

// ---- helpers ----

func objects(v any) []map[string]any {
	list, _ := v.([]any)
	out := make([]map[string]any, 0, len(list))
	for _, e := range list {
		if o, ok := e.(map[string]any); ok {
			out = append(out, o)
		}
	}

	return out
}

func asObject(v any) map[string]any {
	o, _ := v.(map[string]any)

	return o
}

func str(o map[string]any, key string) string {
	switch v := o[key].(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func number(v any) float64 {
	var s string
	switch n := v.(type) {
	case json.Number:
		s = n.String()
	case string:
		s = n
	default:
		return 0
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}

	return f
}

// distinct lists the distinct non-empty values of key, in order; n 0 means
// no cap.
func distinct(list []map[string]any, key string, n int) []string {
	seen := map[string]bool{}
	var out []string
	for _, o := range list {
		v := str(o, key)
		if v == "" || seen[v] {
			continue
		}

		seen[v] = true
		out = append(out, v)
		if n > 0 && len(out) == n {
			break
		}
	}

	return out
}

func queryLimit(req Request, fallback, ceiling int) int {
	limit, err := strconv.Atoi(req.Query.Get("limit"))
	if err != nil || limit <= 0 {
		return fallback
	}

	return min(limit, ceiling)
}

func truncateToSeconds(v any) (string, bool) {
	var s string
	switch t := v.(type) {
	case string:
		s = t
	case json.Number:
		s = t.String()
	default:
		return "", false
	}

	ms, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return "", false
	}

	return strconv.FormatInt(ms/1000, 10), true
}

func tradeSeconds(v any, key string) int64 {
	s, err := strconv.ParseInt(str(asObject(v), key), 10, 64)
	if err != nil {
		return 0
	}

	return s
}

// sortTrades orders trades by time (newest first), price, base volume, maker,
// taker, then their whole JSON, so the same set always comes out the same.
func sortTrades(list []any, timeKey string) {
	slices.SortStableFunc(list, func(a, b any) int {
		if ta, tb := tradeSeconds(a, timeKey), tradeSeconds(b, timeKey); ta != tb {
			if ta > tb {
				return -1
			}

			return 1
		}

		oa, ob := asObject(a), asObject(b)
		for _, key := range []string{"price", "base_volume", "maker", "taker"} {
			if c := strings.Compare(str(oa, key), str(ob, key)); c != 0 {
				return c
			}
		}

		return strings.Compare(canonical(a), canonical(b))
	})
}

func dedupe(list []any) []any {
	out := make([]any, 0, len(list))
	last := ""
	for i, e := range list {
		c := canonical(e)
		if i > 0 && c == last {
			continue
		}

		last = c
		out = append(out, e)
	}

	return out
}

func canonical(v any) string {
	b, _ := json.Marshal(v)

	return string(b)
}

func isZeroCandle(v any, fields []string) bool {
	c, ok := v.(map[string]any)
	if !ok {
		return false
	}

	for _, f := range fields {
		var s string
		switch n := c[f].(type) {
		case string:
			s = n
		case json.Number:
			s = n.String()
		default:
			return false
		}

		if z, err := strconv.ParseFloat(s, 64); err != nil || z != 0 {
			return false
		}
	}

	return true
}
