package compare_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	aggBase  = "http://agg.test"
	scanBase = "http://scan.test/aggregator"
)

type fakeAnswer struct {
	status int
	body   string
}

// fakeDoer answers by full URL. Each URL holds a queue of answers; the last
// one is repeated. Unknown URLs answer 404.
type fakeDoer struct {
	mu      sync.Mutex
	answers map[string][]fakeAnswer
	calls   []string
	agents  []string
}

func newFakeDoer() *fakeDoer {
	return &fakeDoer{answers: map[string][]fakeAnswer{}}
}

func (d *fakeDoer) on(base, path string, query url.Values, answers ...fakeAnswer) {
	u := base + path
	if q := query.Encode(); q != "" {
		u += "?" + q
	}

	d.answers[u] = append(d.answers[u], answers...)
}

func (d *fakeDoer) both(path string, query url.Values, status int, body string) {
	d.on(aggBase, path, query, fakeAnswer{status, body})
	d.on(scanBase, path, query, fakeAnswer{status, body})
}

func (d *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	u := req.URL.String()
	d.calls = append(d.calls, u)
	d.agents = append(d.agents, req.Header.Get("User-Agent"))

	a := fakeAnswer{status: http.StatusNotFound, body: `{"message":"Not Found"}`}
	if q, ok := d.answers[u]; ok && len(q) > 0 {
		a = q[0]
		if len(q) > 1 {
			d.answers[u] = q[1:]
		}
	}

	return &http.Response{
		StatusCode: a.status,
		Body:       io.NopCloser(strings.NewReader(a.body)),
		Header:     http.Header{},
	}, nil
}

// fakeClock moves only when slept on.
type fakeClock struct {
	now    time.Time
	sleeps []time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.sleeps = append(c.sleeps, d)
	c.now = c.now.Add(d)

	return nil
}

// fakeSource is the aggregator as seen by a sample set.
type fakeSource map[string]string

func (s fakeSource) Get(_ context.Context, path string, query url.Values) (any, error) {
	key := path
	if q := query.Encode(); q != "" {
		key += "?" + q
	}

	body, ok := s[key]
	if !ok {
		return nil, &url.Error{Op: "GET", URL: key, Err: io.EOF}
	}

	return decodeJSON(body), nil
}

func decodeJSON(s string) any {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		panic(err)
	}

	return v
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}
