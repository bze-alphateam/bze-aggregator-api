package compare

import (
	"strings"
	"time"
)

// v8.0.x emitted every trade event twice between heights 20,237,800 and
// 22,551,809 (these are their block times, in whole seconds). The aggregator
// lists those trades twice, Scan once (BFE-302).
var (
	doubledFrom = time.Date(2025, 11, 27, 13, 31, 20, 0, time.UTC).Unix()
	doubledTo   = time.Date(2026, 4, 28, 15, 52, 20, 0, time.UTC).Unix()
)

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
	if list, ok := v.([]any); ok {
		prepareTrades(list, defaultTrade)
	}

	return v
}

func normaliseHistory(req Request, v any) any {
	if req.Query.Get("format") != formatCoingecko {
		if list, ok := v.([]any); ok {
			prepareTrades(list, defaultTrade)
		}

		return v
	}

	if o, ok := v.(map[string]any); ok {
		for _, side := range coingeckoSides {
			if list, ok := o[side].([]any); ok {
				prepareTrades(list, coingeckoTrade)
			}
		}
	}

	return v
}

// prepareTrades rewrites one answer's trades in place (BFE-301, BFE-302).
func prepareTrades(list []any, f tradeFields) {
	for _, e := range list {
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
	sortTrades(list, f.time)
}

func reconcileSwaps(_ Request, agg, scan any) (any, any) {
	a, aok := agg.([]any)
	s, sok := scan.([]any)
	if !aok || !sok {
		return agg, scan
	}

	aggFull, scanFull := len(a) >= swapsLimit, len(s) >= swapsLimit

	// the aggregator splits the pool id on "_" and drops a swap whose denoms
	// contain one; Scan lists it (BFE-301)
	s = filterTrades(s, func(t map[string]any) bool {
		return !strings.Contains(str(t, "base"), "_") && !strings.Contains(str(t, "quote"), "_")
	})

	ra, rs := reconcileTrades([][]any{a}, [][]any{s}, defaultTrade, aggFull, scanFull)

	return ra[0], rs[0]
}

var coingeckoSides = []string{"buy", "sell"}

func reconcileHistory(req Request, agg, scan any) (any, any) {
	limit := queryLimit(req, historyDefaultLimit, historyMaxLimit)

	if req.Query.Get("format") != formatCoingecko {
		a, aok := agg.([]any)
		s, sok := scan.([]any)
		if !aok || !sok {
			return agg, scan
		}

		ra, rs := reconcileTrades([][]any{a}, [][]any{s}, defaultTrade, len(a) >= limit, len(s) >= limit)

		return ra[0], rs[0]
	}

	a, aok := agg.(map[string]any)
	s, sok := scan.(map[string]any)
	if !aok || !sok {
		return agg, scan
	}

	// the limit cuts the trades before they are split by side, so both sides
	// make up one page
	al, sl := sideLists(a), sideLists(s)
	ra, rs := reconcileTrades(al, sl, coingeckoTrade, countTrades(al) >= limit, countTrades(sl) >= limit)
	setSideLists(a, ra)
	setSideLists(s, rs)

	return a, s
}

func sideLists(o map[string]any) [][]any {
	lists := make([][]any, len(coingeckoSides))
	for i, side := range coingeckoSides {
		lists[i], _ = o[side].([]any)
	}

	return lists
}

func setSideLists(o map[string]any, lists [][]any) {
	for i, side := range coingeckoSides {
		if _, ok := o[side]; ok {
			o[side] = lists[i]
		}
	}
}

func countTrades(lists [][]any) int {
	n := 0
	for _, l := range lists {
		n += len(l)
	}

	return n
}

// reconcileTrades applies the allowed differences of two pages of trades, each
// made of lists sorted by prepareTrades (the CoinGecko sides, or one list).
func reconcileTrades(agg, scan [][]any, f tradeFields, aggFull, scanFull bool) ([][]any, [][]any) {
	// a full page's oldest second can be cut anywhere by the limit, and once
	// one side has extra rows the two pages end in different seconds: compare
	// only the seconds newer than every full page's oldest one
	cut, trim := int64(0), false
	for _, side := range []struct {
		lists [][]any
		full  bool
	}{{agg, aggFull}, {scan, scanFull}} {
		if oldest, ok := oldestSecond(side.lists, f.time); side.full && ok && (!trim || oldest > cut) {
			cut, trim = oldest, true
		}
	}

	ra := make([][]any, len(agg))
	rs := make([][]any, len(scan))
	for i := range agg {
		a, s := agg[i], []any(nil)
		if i < len(scan) {
			s = scan[i]
		}

		if trim {
			a, s = newerThan(a, cut, f.time), newerThan(s, cut, f.time)
		}

		ra[i], rs[i] = reconcileSeconds(a, s, f.time)
	}

	for i := len(agg); i < len(scan); i++ {
		rs[i] = scan[i]
	}

	return ra, rs
}

func oldestSecond(lists [][]any, timeKey string) (int64, bool) {
	oldest, found := int64(0), false
	for _, l := range lists {
		if len(l) == 0 {
			continue
		}

		if s := tradeSeconds(l[len(l)-1], timeKey); !found || s < oldest {
			oldest, found = s, true
		}
	}

	return oldest, found
}

func newerThan(list []any, cut int64, timeKey string) []any {
	out := make([]any, 0, len(list))
	for _, t := range list {
		if tradeSeconds(t, timeKey) > cut {
			out = append(out, t)
		}
	}

	return out
}

// reconcileSeconds compares the two lists second by second, for the seconds
// both have:
//   - inside the v8.0.x window, a trade the aggregator lists exactly twice as
//     often as Scan counts as Scan's (doubled events);
//   - Scan may list more trades in a second than the aggregator (it drops
//     some at its sync page boundaries); then Scan's second counts as the
//     aggregator's.
//
// Anything else, a second only one side has included, is left for the diff.
func reconcileSeconds(agg, scan []any, timeKey string) ([]any, []any) {
	ag, sg := groupSeconds(agg, timeKey), groupSeconds(scan, timeKey)
	scanBySecond := map[int64][]any{}
	for _, g := range sg {
		scanBySecond[g.second] = g.trades
	}

	replaced := map[int64][]any{}
	var outAgg []any
	for _, g := range ag {
		trades := g.trades
		if st, ok := scanBySecond[g.second]; ok {
			trades = undouble(trades, st, g.second)
			if covers(st, trades) {
				replaced[g.second] = trades
			}
		}

		outAgg = append(outAgg, trades...)
	}

	var outScan []any
	for _, g := range sg {
		if trades, ok := replaced[g.second]; ok {
			outScan = append(outScan, trades...)
			continue
		}

		outScan = append(outScan, g.trades...)
	}

	return outAgg, outScan
}

type secondGroup struct {
	second int64
	trades []any
}

// groupSeconds splits a list sorted newest first into its seconds.
func groupSeconds(list []any, timeKey string) []secondGroup {
	var groups []secondGroup
	for _, t := range list {
		s := tradeSeconds(t, timeKey)
		if n := len(groups); n > 0 && groups[n-1].second == s {
			groups[n-1].trades = append(groups[n-1].trades, t)
			continue
		}

		groups = append(groups, secondGroup{second: s, trades: []any{t}})
	}

	return groups
}

func counts(list []any) map[string]int {
	c := map[string]int{}
	for _, t := range list {
		c[canonical(t)]++
	}

	return c
}

// undouble keeps one of each pair of a trade the aggregator lists twice as
// often as Scan, inside the v8.0.x window only.
func undouble(agg, scan []any, second int64) []any {
	if second < doubledFrom || second > doubledTo {
		return agg
	}

	ac, sc := counts(agg), counts(scan)
	keep := map[string]int{}
	for k, n := range ac {
		keep[k] = n
		if m := sc[k]; m > 0 && n == 2*m {
			keep[k] = m
		}
	}

	out := make([]any, 0, len(agg))
	for _, t := range agg {
		k := canonical(t)
		if keep[k] > 0 {
			keep[k]--
			out = append(out, t)
		}
	}

	return out
}

// covers tells whether scan holds every trade of agg, at least as often.
func covers(scan, agg []any) bool {
	sc := counts(scan)
	for k, n := range counts(agg) {
		if sc[k] < n {
			return false
		}
	}

	return true
}

func filterTrades(list []any, keep func(map[string]any) bool) []any {
	out := make([]any, 0, len(list))
	for _, e := range list {
		if t, ok := e.(map[string]any); !ok || keep(t) {
			out = append(out, e)
		}
	}

	return out
}
