package compare_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/service/compare"
)

// fixed is an endpoint with a fixed sample set and the tickers' normaliser.
func fixed(t *testing.T, reqs ...compare.Request) compare.Endpoint {
	t.Helper()

	ep := endpoint(t, "tickers")
	ep.Sample = func(context.Context, compare.Source, time.Time) ([]compare.Request, error) {
		return reqs, nil
	}

	return ep
}

var tickersReq = compare.Request{Path: "/api/dex/tickers", Query: url.Values{}}

func run(t *testing.T, doer compare.Doer, clock compare.Clock, cfg compare.Config, eps ...compare.Endpoint) (compare.Result, string) {
	t.Helper()

	if cfg.AggBase == "" {
		cfg.AggBase, cfg.ScanBase = aggBase, scanBase
	}

	if cfg.Interval == 0 {
		cfg.Interval, cfg.Duration = time.Minute, time.Minute
	}

	var out bytes.Buffer
	r, err := compare.NewRunner(doer, clock, &out, cfg)
	if err != nil {
		t.Fatal(err)
	}

	res, err := r.Run(context.Background(), eps)
	if err != nil {
		t.Fatal(err)
	}

	return res, out.String()
}

func TestNewRunnerValidates(t *testing.T) {
	if _, err := compare.NewRunner(nil, newFakeClock(), &bytes.Buffer{}, compare.Config{Interval: 1, Duration: 1}); err == nil {
		t.Fatal("a doer is required")
	}

	if _, err := compare.NewRunner(newFakeDoer(), newFakeClock(), &bytes.Buffer{}, compare.Config{}); err == nil {
		t.Fatal("interval and duration are required")
	}
}

func TestSamplingRule(t *testing.T) {
	x, y, z := `[{"market_id":"a","last_price":1}]`, `[{"market_id":"a","last_price":2}]`, `[{"market_id":"a","last_price":3}]`

	cases := []struct {
		name      string
		agg       []string
		scan      string
		divergent bool
	}{
		{"scan equals the first aggregator answer", []string{x, y}, x, false},
		{"scan equals the second aggregator answer", []string{y, x}, x, false},
		{"scan equals neither", []string{y, z}, x, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doer := newFakeDoer()
			for _, a := range c.agg {
				doer.on(aggBase, tickersReq.Path, nil, fakeAnswer{http.StatusOK, a})
			}
			doer.on(scanBase, tickersReq.Path, nil, fakeAnswer{http.StatusOK, c.scan})

			res, out := run(t, doer, newFakeClock(), compare.Config{}, fixed(t, tickersReq))
			if res.Diverged() != c.divergent {
				t.Fatalf("diverged = %v, output:\n%s", res.Diverged(), out)
			}

			want := "round 1 tickers: 1 requests, 0 divergent\n"
			if c.divergent {
				want = "round 1 tickers: 1 requests, 1 divergent\n" +
					"  tickers /api/dex/tickers? $[0].last_price: aggregator=2 scan=1\n"
			}

			if !strings.HasPrefix(out, want) {
				t.Fatalf("unexpected output:\n%s", out)
			}

			// sequential: aggregator, Scan, aggregator again
			wantCalls := []string{aggBase + tickersReq.Path, scanBase + tickersReq.Path, aggBase + tickersReq.Path}
			if !slices.Equal(doer.calls, wantCalls) {
				t.Fatalf("calls %v", doer.calls)
			}

			for _, a := range doer.agents {
				if a != compare.UserAgent {
					t.Fatalf("user agent %q", a)
				}
			}
		})
	}
}

func TestStatusHandling(t *testing.T) {
	cases := []struct {
		name      string
		agg, scan fakeAnswer
		divergent bool
		line      string
	}{
		{"200 against 500", fakeAnswer{200, `[]`}, fakeAnswer{500, `{"message":"x"}`}, true, "? status: aggregator=200 scan=500"},
		{"500 on both", fakeAnswer{500, `{"message":"x"}`}, fakeAnswer{500, `{"message":"x"}`}, true, "status: aggregator=500 scan=500"},
		{"400 on both, equal bodies", fakeAnswer{400, `{"message":"bad"}`}, fakeAnswer{400, `{"message":"bad"}`}, false, ""},
		{"4xx on both, equal bodies", fakeAnswer{404, `{"message":"bad"}`}, fakeAnswer{400, `{"message":"bad"}`}, false, ""},
		{"400 on both, other bodies", fakeAnswer{400, `{"message":"bad"}`}, fakeAnswer{400, `{"message":"worse"}`}, true, `$.message: aggregator="bad" scan="worse"`},
		{"non-JSON bodies", fakeAnswer{200, `0.5`}, fakeAnswer{200, `not json`}, true, "body: aggregator=0.5 scan=not json"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doer := newFakeDoer()
			doer.on(aggBase, tickersReq.Path, nil, c.agg)
			doer.on(scanBase, tickersReq.Path, nil, c.scan)

			res, out := run(t, doer, newFakeClock(), compare.Config{}, fixed(t, tickersReq))
			if res.Diverged() != c.divergent {
				t.Fatalf("diverged = %v, output:\n%s", res.Diverged(), out)
			}

			if c.line != "" && !strings.Contains(out, c.line) {
				t.Fatalf("missing %q in:\n%s", c.line, out)
			}
		})
	}
}

type failingDoer struct{}

func (failingDoer) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

func TestTransportErrorDiverges(t *testing.T) {
	res, out := run(t, failingDoer{}, newFakeClock(), compare.Config{}, fixed(t, tickersReq))
	if !res.Diverged() || !strings.Contains(out, "status: aggregator=error: ") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestSeventyTwoRounds(t *testing.T) {
	doer := newFakeDoer()
	doer.both(tickersReq.Path, nil, http.StatusOK, `[]`)
	clock := newFakeClock()
	start := clock.Now()

	res, out := run(t, doer, clock, compare.Config{Interval: 20 * time.Minute, Duration: 24 * time.Hour}, fixed(t, tickersReq))
	if res.Diverged() {
		t.Fatalf("unexpected divergence:\n%s", out)
	}

	if s := res.Summaries[0]; s.Rounds != 72 || s.Requests != 72 || s.Divergent != 0 {
		t.Fatalf("unexpected summary %+v", s)
	}

	if len(clock.sleeps) != 71 || clock.Now().Sub(start) != 71*20*time.Minute {
		t.Fatalf("slept %d times, until %v", len(clock.sleeps), clock.Now().Sub(start))
	}

	if !strings.Contains(out, "round 72 tickers: 1 requests, 0 divergent\n") ||
		!strings.HasSuffix(out, "summary tickers: 72 rounds, 72 requests, 0 divergent, 0 rounds diverged\n") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestLineCap(t *testing.T) {
	doer := newFakeDoer()
	var reqs []compare.Request
	for i := range 30 {
		q := url.Values{"i": {fmt.Sprint(i)}}
		reqs = append(reqs, compare.Request{Path: tickersReq.Path, Query: q})
		doer.on(aggBase, tickersReq.Path, q, fakeAnswer{200, `[{"market_id":"a","last_price":1}]`})
		doer.on(scanBase, tickersReq.Path, q, fakeAnswer{200, `[{"market_id":"a","last_price":2}]`})
	}

	res, out := run(t, doer, newFakeClock(), compare.Config{}, fixed(t, reqs...))
	if !res.Diverged() || res.Summaries[0].Divergent != 30 {
		t.Fatalf("unexpected result %+v", res)
	}

	details := 0
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "  tickers ") {
			details++
		}
	}

	if details != compare.MaxLines || !strings.Contains(out, "round 1: 10 more difference lines not shown\n") {
		t.Fatalf("%d detail lines in:\n%s", details, out)
	}
}

func TestSamplingErrorDiverges(t *testing.T) {
	ep := fixed(t)
	ep.Sample = func(context.Context, compare.Source, time.Time) ([]compare.Request, error) {
		return nil, errors.New("tickers unavailable")
	}

	res, out := run(t, newFakeDoer(), newFakeClock(), compare.Config{}, ep)
	if !res.Diverged() || !strings.Contains(out, "round 1 tickers: sampling failed: tickers unavailable\n") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestCancelledRunPrintsSummary(t *testing.T) {
	doer := newFakeDoer()
	doer.both(tickersReq.Path, nil, http.StatusOK, `[]`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	r, err := compare.NewRunner(doer, newFakeClock(), &out, compare.Config{
		AggBase: aggBase, ScanBase: scanBase, Interval: time.Minute, Duration: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.Run(ctx, []compare.Endpoint{fixed(t, tickersReq)}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if !strings.Contains(out.String(), "summary tickers: 0 rounds") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}

func TestCommandExitCodes(t *testing.T) {
	clean := newFakeDoer()
	clean.both(tickersReq.Path, nil, http.StatusOK, `[]`)
	clean.both(tickersReq.Path, url.Values{"format": {"coingecko"}}, http.StatusOK, `[]`)

	dirty := newFakeDoer()
	dirty.both(tickersReq.Path, nil, http.StatusOK, `[]`)
	dirty.on(aggBase, tickersReq.Path, url.Values{"format": {"coingecko"}}, fakeAnswer{200, `[]`})
	dirty.on(scanBase, tickersReq.Path, url.Values{"format": {"coingecko"}}, fakeAnswer{200, `[{"ticker_id":"a"}]`})

	for name, c := range map[string]struct {
		doer *fakeDoer
		want error
	}{"clean": {clean, nil}, "diverged": {dirty, compare.ErrDiverged}} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := compare.NewCommand(c.doer, newFakeClock(), &out)
			cmd.SetArgs([]string{"tickers", "--agg", aggBase, "--scan", scanBase, "--duration", "1m"})
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			if err := cmd.Execute(); !errors.Is(err, c.want) {
				t.Fatalf("expected %v, got %v; output:\n%s", c.want, err, out.String())
			}
		})
	}
}

func TestCommandRejectsUnknownEndpoint(t *testing.T) {
	cmd := compare.NewCommand(newFakeDoer(), newFakeClock(), &bytes.Buffer{})
	cmd.SetArgs([]string{"prices"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err == nil || errors.Is(err, compare.ErrDiverged) {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestIntervalsPoolWithoutSwap(t *testing.T) {
	ep := endpoint(t, "intervals")
	sample := func(id string) compare.Endpoint {
		e := ep
		e.Sample = func(context.Context, compare.Source, time.Time) ([]compare.Request, error) {
			return []compare.Request{{Path: "/api/dex/intervals", Query: url.Values{"market_id": {id}, "minutes": {"60"}}}}, nil
		}

		return e
	}

	zero := `[{"start_at":"2026-10-29T12:00:00+02:00","end_at":"2026-10-29T12:59:59+02:00","lowest_price":"0","open_price":"0","average_price":"0","highest_price":"0","close_price":"0","base_volume":"0","quote_volume":"0"}]`
	unknown := fakeAnswer{400, `{"message":"Unknown error!"}`}

	cases := []struct {
		name      string
		id        string
		agg       fakeAnswer
		divergent bool
	}{
		{"pool, aggregator answers []", "ibc/AB_ubze", fakeAnswer{200, `[]`}, false},
		{"pool, aggregator answers one zero candle", "ibc/AB_ubze", fakeAnswer{200, zero}, false},
		{"pool, aggregator has candles", "ibc/AB_ubze", fakeAnswer{200, `[{"start_at":"2026-10-29T12:00:00Z","close_price":"1"}]`}, true},
		{"order book market", "ubze/uvdl", fakeAnswer{200, `[]`}, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := url.Values{"market_id": {c.id}, "minutes": {"60"}}
			doer := newFakeDoer()
			doer.on(aggBase, "/api/dex/intervals", q, c.agg)
			doer.on(scanBase, "/api/dex/intervals", q, unknown)

			res, out := run(t, doer, newFakeClock(), compare.Config{}, sample(c.id))
			if res.Diverged() != c.divergent {
				t.Fatalf("diverged = %v, output:\n%s", res.Diverged(), out)
			}
		})
	}
}

func TestShiftedListPrintsFirstDifferenceOnly(t *testing.T) {
	var agg, scan []string
	for i := range 50 {
		agg = append(agg, fmt.Sprintf(`{"market_id":"m","last_price":%d}`, i))
	}
	// Scan has one extra entry at the front: every index shifts
	scan = append([]string{`{"market_id":"m","last_price":-1}`}, agg...)

	doer := newFakeDoer()
	doer.on(aggBase, tickersReq.Path, nil, fakeAnswer{200, "[" + strings.Join(agg, ",") + "]"})
	doer.on(scanBase, tickersReq.Path, nil, fakeAnswer{200, "[" + strings.Join(scan, ",") + "]"})

	_, out := run(t, doer, newFakeClock(), compare.Config{}, fixed(t, tickersReq))

	details := 0
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "  tickers ") {
			details++
		}
	}

	if details != 2 || !strings.Contains(out, "$.length: aggregator=50 scan=51") ||
		!strings.Contains(out, "$[0].last_price: aggregator=0 scan=-1") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestCommandStoppedCleanExitsZero(t *testing.T) {
	doer := newFakeDoer()
	doer.both(tickersReq.Path, nil, http.StatusOK, `[]`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	cmd := compare.NewCommand(doer, newFakeClock(), &out)
	cmd.SetArgs([]string{"tickers", "--agg", aggBase, "--scan", scanBase})
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("a stopped clean run must exit 0, got %v:\n%s", err, out.String())
	}

	if !strings.Contains(out.String(), "summary tickers: 0 rounds") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}
