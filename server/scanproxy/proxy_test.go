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

// newServer mirrors the aggregator router: CORS, local health routes, and the
// proxy on the /api wildcard.
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

	local := func(c echo.Context) error {
		return c.String(http.StatusOK, "local "+c.Path())
	}
	e.GET("/api/health/market", local)
	e.GET("/api/health/nodes", local)
	e.POST("/api/health/balances", local)
	e.GET("/api/*", p.Handler())

	return e
}

func serve(e *echo.Echo, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(method, target, nil))

	return rec
}

func TestRequestIsAnsweredFromScan(t *testing.T) {
	scan, seen := newScan(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.Header().Set("Access-Control-Allow-Origin", "https://scan.example")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"ticker_id":"scan"}]`))
	})
	logger := &fakeLogger{}
	e := newServer(t, scanproxy.Config{Host: scan.URL + "/aggregator/"}, logger)

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

func TestEveryAPIPathIsProxied(t *testing.T) {
	scan, seen := newScan(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("scan"))
	})
	e := newServer(t, scanproxy.Config{Host: scan.URL}, &fakeLogger{})

	paths := []string{
		"/api/prices", "/api/supply/total", "/api/supply/circulating", "/api/articles/medium",
		"/api/dex/orders", "/api/dex/swaps", "/api/dex/history", "/api/dex/intervals", "/api/dex/tickers",
		"/api/health/aggregator",
	}
	for _, path := range paths {
		if got := serve(e, http.MethodGet, path).Body.String(); got != "scan" {
			t.Fatalf("%s answered %q, want scan", path, got)
		}
	}
	if len(*seen) != len(paths) {
		t.Fatalf("scan saw %d requests, want %d", len(*seen), len(paths))
	}
}

func TestScanStatusIsPassedThrough(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusNotFound, http.StatusBadGateway} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			scan, _ := newScan(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"message":"from scan"}`))
			})
			logger := &fakeLogger{}
			e := newServer(t, scanproxy.Config{Host: scan.URL}, logger)

			rec := serve(e, http.MethodGet, "/api/dex/history?market_id=")

			if rec.Code != status || rec.Body.String() != `{"message":"from scan"}` {
				t.Fatalf("got %d %q, want Scan's %d", rec.Code, rec.Body.String(), status)
			}
			if logger.count() != 0 {
				t.Fatalf("warnings = %v, want none", logger.warnings)
			}
		})
	}
}

func TestNoAnswerFromScanIs502(t *testing.T) {
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
		"timeout":          {Host: slow.URL, Timeout: 50 * time.Millisecond},
		"connection error": {Host: closedURL},
	} {
		t.Run(name, func(t *testing.T) {
			logger := &fakeLogger{}
			e := newServer(t, cfg, logger)

			rec := serve(e, http.MethodGet, "/api/dex/orders?market_id=a/b")

			if rec.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want 502", rec.Code)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != `{"message":"upstream unavailable"}` {
				t.Fatalf("body = %q", got)
			}
			if logger.count() != 1 {
				t.Fatalf("warnings = %v, want exactly one", logger.warnings)
			}
			if !strings.Contains(logger.warnings[0], "/api/dex/orders") {
				t.Fatalf("warning %q does not name the path", logger.warnings[0])
			}
		})
	}
}

func TestHealthEndpointsStayLocal(t *testing.T) {
	scan, seen := newScan(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("scan"))
	})
	e := newServer(t, scanproxy.Config{Host: scan.URL}, &fakeLogger{})

	for _, path := range []string{"/api/health/market", "/api/health/nodes"} {
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

func TestNewRequiresAHost(t *testing.T) {
	for name, tc := range map[string]struct {
		host    string
		wantErr bool
	}{
		"empty":       {host: "", wantErr: true},
		"blank":       {host: "  ", wantErr: true},
		"only slash":  {host: "/", wantErr: true},
		"with prefix": {host: "https://scan.example/aggregator"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := scanproxy.New(scanproxy.Config{Host: tc.host}, &http.Client{}, &fakeLogger{})
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
