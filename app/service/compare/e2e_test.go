package compare_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bze-alphateam/bze-aggregator-api/app/service/compare"
)

// The e2e tests run the real command, with the real HTTP client, against two
// local servers playing the aggregator and BZE Scan, from a directory without
// a .env, a database or a node.

var aggFixtures = map[string]string{
	"/api/dex/tickers": `[
		{"base":"ubze","quote":"uvdl","market_id":"ubze/uvdl","base_volume":10,"last_price":0.5},
		{"base":"ibc/AB","quote":"ubze","market_id":"ibc/AB_ubze","base_volume":0,"last_price":2}]`,
	"/api/dex/tickers?format=coingecko": `[
		{"ticker_id":"ubze_uvdl","pool_id":"ubze/uvdl","last_price":0.5},
		{"ticker_id":"lp_ibc/AB_ubze","pool_id":"lp_ibc/AB_ubze","liquidity_in_usd":1234.5}]`,
	"/api/dex/orders?depth=0&market_id=ubze%2Fuvdl":                `{"market_id":"ubze/uvdl","timestamp":"2026-10-29 12:00:00","bids":null,"asks":[{"price":"1","volume":"2"}]}`,
	"/api/dex/orders?depth=10&market_id=ubze%2Fuvdl":               `{"market_id":"ubze/uvdl","timestamp":"2026-10-29 12:00:00","bids":null,"asks":[{"price":"1","volume":"2"}]}`,
	"/api/dex/orders?depth=0&format=coingecko&ticker_id=ubze_uvdl": `{"ticker_id":"ubze_uvdl","timestamp":"2026-10-29 12:00:00","bids":null,"asks":[["1","2"]]}`,
	"/api/dex/history?limit=50&market_id=ubze%2Fuvdl":              `[{"order_id":9,"price":"0.5","base_volume":"2","quote_volume":"1","executed_at":"1793271600000","order_type":"sell","maker":"bze1m","taker":"bze1t"}]`,
	"/api/dex/swaps?address=":                                      `[{"order_id":1,"price":"2","base_volume":"1","quote_volume":"2","executed_at":"1793275200000","order_type":"buy","maker":"","taker":"bze1a","pool_id":"ibc/AB_ubze","base":"ibc/AB","quote":"ubze"}]`,
	"/api/dex/swaps?address=bze1a":                                 `[{"order_id":1,"price":"2","base_volume":"1","quote_volume":"2","executed_at":"1793275200000","order_type":"buy","maker":"","taker":"bze1a","pool_id":"ibc/AB_ubze","base":"ibc/AB","quote":"ubze"}]`,
	"/api/dex/intervals?limit=100&market_id=ubze%2Fuvdl&minutes=60": `[
		{"market_id":"ubze/uvdl","minutes":60,"start_at":"2026-10-29T12:00:00+02:00","end_at":"2026-10-29T12:59:59+02:00","lowest_price":"0","open_price":"0","average_price":"0","highest_price":"0","close_price":"0","base_volume":"0","quote_volume":"0"},
		{"market_id":"ubze/uvdl","minutes":60,"start_at":"2026-10-29T13:00:00+02:00","end_at":"2026-10-29T13:59:59+02:00","lowest_price":"1","open_price":"1","average_price":"1","highest_price":"1","close_price":"1","base_volume":"1","quote_volume":"1"}]`,
}

var scanFixtures = map[string]string{
	"/api/dex/tickers": `[
		{"base":"ibc/AB","quote":"ubze","market_id":"ibc/AB_ubze","base_volume":0,"last_price":2},
		{"base":"ubze","quote":"uvdl","market_id":"ubze/uvdl","base_volume":10,"last_price":0.5}]`,
	"/api/dex/tickers?format=coingecko": `[
		{"ticker_id":"lp_ibc/AB_ubze","pool_id":"lp_ibc/AB_ubze","liquidity_in_usd":1234.5001},
		{"ticker_id":"ubze_uvdl","pool_id":"ubze/uvdl","last_price":0.5}]`,
	"/api/dex/orders?depth=0&market_id=ubze%2Fuvdl":                `{"market_id":"ubze/uvdl","timestamp":"1793275200","bids":[],"asks":[{"price":"1","volume":"2"}]}`,
	"/api/dex/orders?depth=10&market_id=ubze%2Fuvdl":               `{"market_id":"ubze/uvdl","timestamp":"1793275200","bids":[],"asks":[{"price":"1","volume":"2"}]}`,
	"/api/dex/orders?depth=0&format=coingecko&ticker_id=ubze_uvdl": `{"ticker_id":"ubze_uvdl","timestamp":"1793275200","bids":[],"asks":[["1","2"]]}`,
	"/api/dex/history?limit=50&market_id=ubze%2Fuvdl":              `[{"order_id":1793271600000003,"price":"0.5","base_volume":"2","quote_volume":"1","executed_at":"1793271600250","order_type":"sell","maker":"bze1m","taker":"bze1t"}]`,
	"/api/dex/swaps?address=bze1a":                                 `[{"order_id":1793275200000001,"price":"2","base_volume":"1","quote_volume":"2","executed_at":"1793275200421","order_type":"buy","maker":"","taker":"bze1a","pool_id":"ibc/AB_ubze","base":"ibc/AB","quote":"ubze"}]`,
	"/api/dex/intervals?limit=100&market_id=ubze%2Fuvdl&minutes=60": `[
		{"market_id":"ubze/uvdl","minutes":60,"start_at":"2026-10-29T11:00:00Z","end_at":"2026-10-29T11:59:59Z","lowest_price":"1","open_price":"1","average_price":"1","highest_price":"1","close_price":"1","base_volume":"1","quote_volume":"1"}]`,
}

func fixtureServer(t *testing.T, prefix string, fixtures map[string]string, badAgent *atomic.Bool) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != compare.UserAgent {
			badAgent.Store(true)
		}

		key := strings.TrimPrefix(r.URL.Path, prefix)
		if q := r.URL.Query().Encode(); q != "" {
			key += "?" + q
		}

		w.Header().Set("Content-Type", "application/json")
		body, ok := fixtures[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))

			return
		}

		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	return srv
}

// inEmptyDir runs the test from a directory without a .env.
func inEmptyDir(t *testing.T) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.Chdir(wd) })

	if _, err := os.Stat(".env"); !os.IsNotExist(err) {
		t.Fatal("expected no .env")
	}
}

func runAll(t *testing.T, scan map[string]string) (string, error) {
	t.Helper()
	inEmptyDir(t)

	var badAgent atomic.Bool
	agg := fixtureServer(t, "", aggFixtures, &badAgent)
	sc := fixtureServer(t, "/aggregator", scan, &badAgent)

	var out bytes.Buffer
	cmd := compare.NewCommand(compare.NewHTTPClient(), newFakeClock(), &out)
	cmd.SetArgs([]string{"all", "--agg", agg.URL, "--scan", sc.URL + "/aggregator", "--interval", "20m", "--duration", "1h"})
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if badAgent.Load() {
		t.Fatal("a request came without the compare user agent")
	}

	return out.String(), err
}

func TestE2ECompareAllClean(t *testing.T) {
	out, err := runAll(t, scanFixtures)
	if err != nil {
		t.Fatalf("expected a clean run, got %v:\n%s", err, out)
	}

	for _, line := range []string{
		"round 1 orders: 3 requests, 0 divergent\n",
		"round 1 swaps: 1 requests, 0 divergent\n",
		"round 1 history: 6 requests, 0 divergent\n",
		"round 1 intervals: 12 requests, 0 divergent\n",
		"round 1 tickers: 2 requests, 0 divergent\n",
		"round 3 tickers: 2 requests, 0 divergent\n",
		"summary orders: 3 rounds, 9 requests, 0 divergent, 0 rounds diverged\n",
		"summary tickers: 3 rounds, 6 requests, 0 divergent, 0 rounds diverged\n",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("missing %q in:\n%s", line, out)
		}
	}
}

func TestE2ECompareAllDiverged(t *testing.T) {
	scan := map[string]string{}
	for k, v := range scanFixtures {
		scan[k] = v
	}
	scan["/api/dex/orders?depth=10&market_id=ubze%2Fuvdl"] = `{"market_id":"ubze/uvdl","timestamp":"1","bids":[],"asks":[{"price":"1","volume":"3"}]}`

	out, err := runAll(t, scan)
	if !errors.Is(err, compare.ErrDiverged) {
		t.Fatalf("expected ErrDiverged, got %v:\n%s", err, out)
	}

	for _, line := range []string{
		"round 1 orders: 3 requests, 1 divergent\n",
		"  orders /api/dex/orders?depth=10&market_id=ubze%2Fuvdl $.asks[0].volume: aggregator=\"2\" scan=\"3\"\n",
		"summary orders: 3 rounds, 9 requests, 3 divergent, 3 rounds diverged\n",
		"summary swaps: 3 rounds, 3 requests, 0 divergent, 0 rounds diverged\n",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("missing %q in:\n%s", line, out)
		}
	}
}
