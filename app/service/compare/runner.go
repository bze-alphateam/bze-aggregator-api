package compare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// UserAgent is sent with every request.
const UserAgent = "bze-agg-compare"

// MaxLines caps the difference lines printed per round.
const MaxLines = 20

// maxBody caps a response body read.
const maxBody = 64 << 20

// Doer sends an HTTP request; *http.Client is one.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Clock tells the time and waits between rounds.
type Clock interface {
	Now() time.Time
	Sleep(ctx context.Context, d time.Duration) error
}

// Config holds the bases and the schedule of a run.
type Config struct {
	AggBase  string
	ScanBase string
	Interval time.Duration
	Duration time.Duration
}

// Summary is one endpoint's total over a run.
type Summary struct {
	Endpoint       string
	Rounds         int
	Requests       int
	Divergent      int
	RoundsDiverged int
	SampleErrors   int
}

// Result is a run's outcome.
type Result struct {
	Summaries []Summary
}

// Diverged tells whether any round diverged (or failed to sample).
func (r Result) Diverged() bool {
	for _, s := range r.Summaries {
		if s.RoundsDiverged > 0 {
			return true
		}
	}

	return false
}

// Runner compares the endpoints round after round.
type Runner struct {
	doer  Doer
	clock Clock
	out   io.Writer
	cfg   Config
}

// NewRunner builds a runner.
func NewRunner(doer Doer, clock Clock, out io.Writer, cfg Config) (*Runner, error) {
	if doer == nil || clock == nil || out == nil {
		return nil, errors.New("compare: doer, clock and output are required")
	}

	if cfg.Interval <= 0 || cfg.Duration <= 0 {
		return nil, errors.New("compare: interval and duration must be positive")
	}

	return &Runner{doer: doer, clock: clock, out: out, cfg: cfg}, nil
}

// Run compares the endpoints every interval for the duration, the first
// round right away, then prints the summary. A cancelled context stops it
// after the summary, with the context's error.
func (r *Runner) Run(ctx context.Context, endpoints []Endpoint) (Result, error) {
	summaries := make([]Summary, len(endpoints))
	for i, ep := range endpoints {
		summaries[i].Endpoint = ep.Name
	}

	start := r.clock.Now()
	end := start.Add(r.cfg.Duration)

	var err error
	for n := 0; ; n++ {
		planned := start.Add(time.Duration(n) * r.cfg.Interval)
		if n > 0 {
			if !planned.Before(end) {
				break
			}

			if wait := planned.Sub(r.clock.Now()); wait > 0 {
				if err = r.clock.Sleep(ctx, wait); err != nil {
					break
				}
			}
		}

		if err = ctx.Err(); err != nil {
			break
		}

		r.round(ctx, n+1, endpoints, summaries)
	}

	for _, s := range summaries {
		r.printf("summary %s: %d rounds, %d requests, %d divergent, %d rounds diverged",
			s.Endpoint, s.Rounds, s.Requests, s.Divergent, s.RoundsDiverged)
		if s.SampleErrors > 0 {
			r.printf(", %d sampling errors", s.SampleErrors)
		}

		r.printf("\n")
	}

	return Result{Summaries: summaries}, err
}

func (r *Runner) round(ctx context.Context, n int, endpoints []Endpoint, summaries []Summary) {
	now := r.clock.Now()
	src := &memoSource{runner: r, cache: map[string]any{}}
	lines, hidden := 0, 0

	for i, ep := range endpoints {
		s := &summaries[i]
		s.Rounds++

		reqs, err := ep.Sample(ctx, src, now)
		if err != nil {
			s.SampleErrors++
			s.RoundsDiverged++
			r.printf("round %d %s: sampling failed: %v\n", n, ep.Name, err)

			continue
		}

		var details []string
		divergent := 0
		for _, req := range reqs {
			diffs := r.check(ctx, ep, req)
			if len(diffs) == 0 {
				continue
			}

			divergent++
			for _, d := range diffs {
				details = append(details, fmt.Sprintf("  %s %s?%s %s: aggregator=%s scan=%s",
					ep.Name, req.Path, req.Params(), d.Path, d.Aggregator, d.Scan))
			}
		}

		s.Requests += len(reqs)
		s.Divergent += divergent
		if divergent > 0 {
			s.RoundsDiverged++
		}

		r.printf("round %d %s: %d requests, %d divergent\n", n, ep.Name, len(reqs), divergent)
		for _, d := range details {
			if lines == MaxLines {
				hidden++
				continue
			}

			lines++
			r.printf("%s\n", d)
		}
	}

	if hidden > 0 {
		r.printf("round %d: %d more difference lines not shown\n", n, hidden)
	}
}

// check fetches the aggregator (A1), Scan (S) and the aggregator again (A2).
// The request diverges only when S differs from both, so a trade landing
// between the requests is not a divergence. It returns the smaller of the two
// difference lists, or none.
func (r *Runner) check(ctx context.Context, ep Endpoint, req Request) []Difference {
	a1 := r.fetch(ctx, r.cfg.AggBase, req)
	s := r.fetch(ctx, r.cfg.ScanBase, req)
	a2 := r.fetch(ctx, r.cfg.AggBase, req)

	d1 := comparePair(ep, req, a1, s)
	if len(d1) == 0 {
		return nil
	}

	d2 := comparePair(ep, req, a2, s)
	if len(d2) == 0 {
		return nil
	}

	if len(d2) < len(d1) {
		return d2
	}

	return d1
}

type response struct {
	status int
	body   []byte
	err    error
}

func (resp response) describe() string {
	if resp.err != nil {
		return clip("error: " + resp.err.Error())
	}

	return strconv.Itoa(resp.status)
}

func (resp response) ok() bool {
	return resp.err == nil && resp.status >= 200 && resp.status < 300
}

func (resp response) clientError() bool {
	return resp.err == nil && resp.status >= 400 && resp.status < 500
}

func comparePair(ep Endpoint, req Request, agg, scan response) []Difference {
	switch {
	case agg.ok() && scan.ok():
		av, aerr := decode(agg.body)
		sv, serr := decode(scan.body)
		if aerr != nil || serr != nil {
			return compareRaw(agg.body, scan.body)
		}

		na, ns := ep.Normalise(req, av), ep.Normalise(req, sv)
		if ep.Reconcile != nil {
			na, ns = ep.Reconcile(req, na, ns)
		}

		return Diff(na, ns, ep.Tolerances)
	case agg.clientError() && scan.clientError():
		// a 4xx on both sides is a match when the bodies are equal
		av, aerr := decode(agg.body)
		sv, serr := decode(scan.body)
		if aerr != nil || serr != nil {
			return compareRaw(agg.body, scan.body)
		}

		return Diff(av, sv, nil)
	default:
		if ep.AllowStatus != nil && agg.err == nil && scan.err == nil {
			var na any
			if agg.ok() {
				if av, err := decode(agg.body); err == nil {
					na = ep.Normalise(req, av)
				}
			}

			if ep.AllowStatus(req, na, agg.status, scan.status) {
				return nil
			}
		}

		return []Difference{{Path: "status", Aggregator: agg.describe(), Scan: scan.describe()}}
	}
}

func compareRaw(agg, scan []byte) []Difference {
	if bytes.Equal(bytes.TrimSpace(agg), bytes.TrimSpace(scan)) {
		return nil
	}

	return []Difference{{Path: "body", Aggregator: clip(string(agg)), Scan: clip(string(scan))}}
}

func decode(body []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}

	return v, nil
}

func (r *Runner) fetch(ctx context.Context, base string, req Request) response {
	u := strings.TrimRight(base, "/") + req.Path
	if q := req.Query.Encode(); q != "" {
		u += "?" + q
	}

	hr, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return response{err: err}
	}

	hr.Header.Set("User-Agent", UserAgent)
	hr.Header.Set("Accept", "application/json")

	resp, err := r.doer.Do(hr)
	if err != nil {
		return response{err: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return response{err: err}
	}

	return response{status: resp.StatusCode, body: body}
}

func (r *Runner) printf(format string, args ...any) {
	_, _ = fmt.Fprintf(r.out, format, args...)
}

// memoSource reads the aggregator for the sample sets, each URL once per
// round.
type memoSource struct {
	runner *Runner
	cache  map[string]any
}

func (m *memoSource) Get(ctx context.Context, path string, query url.Values) (any, error) {
	req := Request{Path: path, Query: query}
	key := path + "?" + req.Params()
	if v, ok := m.cache[key]; ok {
		return v, nil
	}

	resp := m.runner.fetch(ctx, m.runner.cfg.AggBase, req)
	if !resp.ok() {
		return nil, fmt.Errorf("aggregator %s: %s", key, resp.describe())
	}

	v, err := decode(resp.body)
	if err != nil {
		return nil, fmt.Errorf("aggregator %s: %w", key, err)
	}

	m.cache[key] = v

	return v, nil
}
