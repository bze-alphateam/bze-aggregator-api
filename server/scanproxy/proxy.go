// Package scanproxy answers the aggregator API from BZE Scan, which serves the
// same paths and responses under its /aggregator base.
package scanproxy

import (
	"context"
	"errors"
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

// Doer sends HTTP requests (*http.Client satisfies it).
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Logger receives one warning per request Scan did not answer.
type Logger interface {
	Warnf(format string, args ...interface{})
}

type Config struct {
	// Host is Scan's base URL including its /aggregator prefix.
	Host string
	// Timeout of one Scan request; zero means DefaultTimeout.
	Timeout time.Duration
}

type Proxy struct {
	host    string
	timeout time.Duration
	doer    Doer
	logger  Logger
}

type errResponse struct {
	Message string `json:"message"`
}

// New requires a Scan host.
func New(cfg Config, doer Doer, logger Logger) (*Proxy, error) {
	p := &Proxy{
		host:    strings.TrimRight(strings.TrimSpace(cfg.Host), "/"),
		timeout: cfg.Timeout,
		doer:    doer,
		logger:  logger,
	}
	if p.host == "" {
		return nil, errors.New("SCAN_API_HOST is empty")
	}
	if p.timeout <= 0 {
		p.timeout = DefaultTimeout
	}

	return p, nil
}

// Handler forwards the request to the same path and query on Scan and copies
// back its status, Content-Type and body, whatever the status. When Scan does
// not answer (transport error or timeout) it responds 502.
func (p *Proxy) Handler() echo.HandlerFunc {
	return func(c echo.Context) error {
		status, contentType, body, err := p.forward(c.Request())
		if err != nil {
			p.logger.Warnf("scan proxy: %s did not get an answer: %v", c.Request().URL.Path, err)
			return c.JSON(http.StatusBadGateway, errResponse{Message: "upstream unavailable"})
		}

		if contentType != "" {
			c.Response().Header().Set(echo.HeaderContentType, contentType)
		}
		c.Response().WriteHeader(status)
		_, err = c.Response().Write(body)

		return err
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", nil, err
	}

	return resp.StatusCode, resp.Header.Get(echo.HeaderContentType), body, nil
}
