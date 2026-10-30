package scanproxy_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/server/scanproxy"
	"github.com/labstack/echo/v4"
)

const allEndpoints = "prices,supply,articles,orders,swaps,history,intervals,tickers"

type fakeLogger struct {
	mx       sync.Mutex
	warnings []string
}

func (l *fakeLogger) Warnf(format string, args ...interface{}) {
	l.mx.Lock()
	defer l.mx.Unlock()
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}

func (l *fakeLogger) count() int {
	l.mx.Lock()
	defer l.mx.Unlock()

	return len(l.warnings)
}

// scanRequest is what the fake Scan saw.
type scanRequest struct {
	uri       string
	userAgent string
}

// newScan starts a fake Scan answering every request with handler and
// recording the requests it received.
func newScan(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]scanRequest) {
	t.Helper()
	var (
		mx   sync.Mutex
		seen []scanRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mx.Lock()
		seen = append(seen, scanRequest{uri: r.URL.RequestURI(), userAgent: r.UserAgent()})
		mx.Unlock()
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	return srv, &seen
}

// newServer mirrors the aggregator router: CORS, the proxy, then local
// handlers answering "local <path>".
func newServer(t *testing.T, cfg scanproxy.Config, logger *fakeLogger) *echo.Echo {
	t.Helper()
	p, err := scanproxy.New(cfg, &http.Client{}, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set(echo.HeaderAccessControlAllowOrigin, "*")
			return next(c)
		}
	})
	e.Use(p.Middleware())

	local := func(c echo.Context) error {
		return c.String(http.StatusOK, "local "+c.Path())
	}
	for _, path := range []string{
		"/api/prices", "/api/supply/total", "/api/supply/circulating", "/api/articles/medium",
		"/api/dex/orders", "/api/dex/swaps", "/api/dex/history", "/api/dex/intervals", "/api/dex/tickers",
		"/api/health/market", "/api/health/aggregator", "/api/health/nodes",
	} {
		e.GET(path, local)
	}
	e.POST("/api/health/balances", local)

	return e
}

func serve(e *echo.Echo, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(method, target, nil))

	return rec
}

func TestListedEndpointIsAnsweredFromScan(t *testing.T) {
	scan, seen := newScan(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.Header().Set("Access-Control-Allow-Origin", "https://scan.example")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"ticker_id":"scan"}]`))
	})
	logger := &fakeLogger{}
	e := newServer(t, scanproxy.Config{Host: scan.URL + "/aggregator/", Endpoints: "tickers"}, logger)

	rec := serve(e, http.MethodGet, "/api/dex/tickers?format=coingecko")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=UTF-8" {
		t.Fatalf("content type = %q", got)
	}
	if got := rec.Body.String(); got != `[{"ticker_id":"scan"}]` {
		t.Fatalf("body = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("CORS header = %q, want the aggregator's own %q", got, "*")
	}
	if len(*seen) != 1 {
		t.Fatalf("scan saw %d requests, want 1", len(*seen))
	}
	if got := (*seen)[0].uri; got != "/aggregator/api/dex/tickers?format=coingecko" {
		t.Fatalf("scan request = %q", got)
	}
	if got := (*seen)[0].userAgent; got != "bze-agg-proxy" {
		t.Fatalf("user agent = %q", got)
	}
	if logger.count() != 0 {
		t.Fatalf("warnings = %v, want none", logger.warnings)
	}
}

func TestEveryNameProxiesItsRoutes(t *testing.T) {
	scan, seen := newScan(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("scan"))
	})
	for name, paths := range map[string][]string{
		"prices":    {"/api/prices"},
		"supply":    {"/api/supply/total", "/api/supply/circulating"},
		"articles":  {"/api/articles/medium"},
		"orders":    {"/api/dex/orders"},
		"swaps":     {"/api/dex/swaps"},
		"history":   {"/api/dex/history"},
		"intervals": {"/api/dex/intervals"},
		"tickers":   {"/api/dex/tickers"},
	} {
		e := newServer(t, scanproxy.Config{Host: scan.URL, Endpoints: name}, &fakeLogger{})
		for _, path := range paths {
			if got := serve(e, http.MethodGet, path).Body.String(); got != "scan" {
				t.Fatalf("%s %s answered %q, want scan", name, path, got)
			}
		}
	}
	if len(*seen) != 9 {
		t.Fatalf("scan saw %d requests, want 9", len(*seen))
	}
}

func TestScanFailuresFallBackToTheLocalHandler(t *testing.T) {
	failing, _ := newScan(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	})
	slow, _ := newScan(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
		_, _ = w.Write([]byte("too late"))
	})
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close()

	for name, cfg := range map[string]scanproxy.Config{
		"5xx":              {Host: failing.URL, Endpoints: "orders"},
		"timeout":          {Host: slow.URL, Endpoints: "orders", Timeout: 50 * time.Millisecond},
		"connection error": {Host: closedURL, Endpoints: "orders"},
	} {
		t.Run(name, func(t *testing.T) {
			logger := &fakeLogger{}
			e := newServer(t, cfg, logger)

			rec := serve(e, http.MethodGet, "/api/dex/orders?market_id=a/b")

			if rec.Code != http.StatusOK || rec.Body.String() != "local /api/dex/orders" {
				t.Fatalf("got %d %q, want the local handler", rec.Code, rec.Body.String())
			}
			if logger.count() != 1 {
				t.Fatalf("warnings = %v, want exactly one", logger.warnings)
			}
			if !strings.Contains(logger.warnings[0], "orders") {
				t.Fatalf("warning %q does not name the endpoint", logger.warnings[0])
			}
		})
	}
}

func TestScan4xxIsPassedThrough(t *testing.T) {
	scan, _ := newScan(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid market_id"}`))
	})
	logger := &fakeLogger{}
	e := newServer(t, scanproxy.Config{Host: scan.URL, Endpoints: "history"}, logger)

	rec := serve(e, http.MethodGet, "/api/dex/history?market_id=")

	if rec.Code != http.StatusBadRequest || rec.Body.String() != `{"message":"invalid market_id"}` {
		t.Fatalf("got %d %q, want Scan's 400", rec.Code, rec.Body.String())
	}
	if logger.count() != 0 {
		t.Fatalf("warnings = %v, want none", logger.warnings)
	}
}

func TestUnlistedAndHealthEndpointsStayLocal(t *testing.T) {
	scan, seen := newScan(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("scan"))
	})

	e := newServer(t, scanproxy.Config{Host: scan.URL, Endpoints: "tickers"}, &fakeLogger{})
	if got := serve(e, http.MethodGet, "/api/prices").Body.String(); got != "local /api/prices" {
		t.Fatalf("unlisted /api/prices answered %q", got)
	}

	e = newServer(t, scanproxy.Config{Host: scan.URL, Endpoints: allEndpoints}, &fakeLogger{})
	for _, path := range []string{"/api/health/market", "/api/health/aggregator", "/api/health/nodes"} {
		if got := serve(e, http.MethodGet, path).Body.String(); got != "local "+path {
			t.Fatalf("%s answered %q", path, got)
		}
	}
	if got := serve(e, http.MethodPost, "/api/health/balances").Body.String(); got != "local /api/health/balances" {
		t.Fatalf("/api/health/balances answered %q", got)
	}

	if len(*seen) != 0 {
		t.Fatalf("scan saw %d requests, want none", len(*seen))
	}
}

func TestEmptyListProxiesNothing(t *testing.T) {
	e := newServer(t, scanproxy.Config{}, &fakeLogger{})
	if got := serve(e, http.MethodGet, "/api/dex/tickers").Body.String(); got != "local /api/dex/tickers" {
		t.Fatalf("answered %q", got)
	}
}

func TestNewValidatesTheConfig(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg     scanproxy.Config
		wantErr bool
	}{
		"unknown name":      {cfg: scanproxy.Config{Host: "https://scan.example/aggregator", Endpoints: "tickers,health"}, wantErr: true},
		"list without host": {cfg: scanproxy.Config{Endpoints: "tickers"}, wantErr: true},
		"empty list":        {cfg: scanproxy.Config{}},
		"host without list": {cfg: scanproxy.Config{Host: "https://scan.example/aggregator"}},
		"all names, spaced": {cfg: scanproxy.Config{Host: "https://scan.example/aggregator", Endpoints: " prices , " + allEndpoints + ","}},
		"only separators":   {cfg: scanproxy.Config{Endpoints: " , "}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := scanproxy.New(tc.cfg, &http.Client{}, &fakeLogger{})
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
