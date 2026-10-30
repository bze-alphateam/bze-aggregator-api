// Package scanproxy answers listed aggregator endpoints from BZE Scan, which
// serves the same paths and responses under its /aggregator base. The
// aggregator's own handler stays the failover when Scan does not answer.
package scanproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

const (
	// DefaultTimeout bounds one request to Scan.
	DefaultTimeout = 10 * time.Second
	// userAgent names the proxy to Scan's edge, which rejects some default
	// client user agents.
	userAgent = "bze-agg-proxy"
)

// vocabulary maps the names SCAN_PROXIED_ENDPOINTS accepts to their routes.
// /api/health/* is deliberately absent: it reports on the aggregator itself.
var vocabulary = map[string][]string{
	"prices":    {"/api/prices"},
	"supply":    {"/api/supply/total", "/api/supply/circulating"},
	"articles":  {"/api/articles/medium"},
	"orders":    {"/api/dex/orders"},
	"swaps":     {"/api/dex/swaps"},
	"history":   {"/api/dex/history"},
	"intervals": {"/api/dex/intervals"},
	"tickers":   {"/api/dex/tickers"},
}

// Doer sends HTTP requests (*http.Client satisfies it).
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Logger receives one warning per failover.
type Logger interface {
	Warnf(format string, args ...interface{})
}

type Config struct {
	// Host is Scan's base URL including its /aggregator prefix.
	Host string
	// Endpoints is the comma-separated list of vocabulary names to proxy.
	Endpoints string
	// Timeout of one Scan request; zero means DefaultTimeout.
	Timeout time.Duration
}

type Proxy struct {
	host    string
	routes  map[string]string // route path -> vocabulary name
	timeout time.Duration
	doer    Doer
	logger  Logger
}

// New validates the config: every name must be in the vocabulary and a
// non-empty list needs a host. An empty list proxies nothing.
func New(cfg Config, doer Doer, logger Logger) (*Proxy, error) {
	p := &Proxy{
		host:    strings.TrimRight(strings.TrimSpace(cfg.Host), "/"),
		routes:  make(map[string]string),
		timeout: cfg.Timeout,
		doer:    doer,
		logger:  logger,
	}
	if p.timeout <= 0 {
		p.timeout = DefaultTimeout
	}

	for _, name := range strings.Split(cfg.Endpoints, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		paths, ok := vocabulary[name]
		if !ok {
			return nil, fmt.Errorf("SCAN_PROXIED_ENDPOINTS: unknown endpoint %q", name)
		}
		for _, path := range paths {
			p.routes[path] = name
		}
	}

	if len(p.routes) > 0 && p.host == "" {
		return nil, fmt.Errorf("SCAN_PROXIED_ENDPOINTS is set but SCAN_API_HOST is empty")
	}

	return p, nil
}

// Middleware forwards requests to listed routes to Scan and copies back its
// status, Content-Type and body. A transport error, timeout or 5xx falls back
// to the local handler; a 4xx is Scan's valid answer and is passed through.
func (p *Proxy) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			name, ok := p.routes[c.Path()]
			if !ok {
				return next(c)
			}

			status, contentType, body, err := p.forward(c.Request())
			if err != nil {
				p.logger.Warnf("scan proxy: %s failed over to the local handler: %v", name, err)
				return next(c)
			}

			if contentType != "" {
				c.Response().Header().Set(echo.HeaderContentType, contentType)
			}
			c.Response().WriteHeader(status)
			_, err = c.Response().Write(body)

			return err
		}
	}
}

func (p *Proxy) forward(in *http.Request) (int, string, []byte, error) {
	ctx, cancel := context.WithTimeout(in.Context(), p.timeout)
	defer cancel()

	target := p.host + in.URL.Path
	if in.URL.RawQuery != "" {
		target += "?" + in.URL.RawQuery
	}

	req, err := http.NewRequestWithContext(ctx, in.Method, target, nil)
	if err != nil {
		return 0, "", nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	if accept := in.Header.Get(echo.HeaderAccept); accept != "" {
		req.Header.Set(echo.HeaderAccept, accept)
	}

	resp, err := p.doer.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return 0, "", nil, fmt.Errorf("scan answered %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", nil, err
	}

	return resp.StatusCode, resp.Header.Get(echo.HeaderContentType), body, nil
}
