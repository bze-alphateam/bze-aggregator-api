package dex

import (
	"math"
	"strings"
	"testing"

	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/sirupsen/logrus"
)

// newTestLogger returns a logger whose output is discarded into a buffer, so the
// services under test can log freely without polluting the test output.
func newTestLogger() logrus.FieldLogger {
	l := logrus.New()
	l.SetOutput(&strings.Builder{})

	return l
}

// assertFloat64 compares two float64 values within a small epsilon; the services
// round-trip decimal strings through cosmossdk math, so exact equality is fragile.
func assertFloat64(t *testing.T, name string, got, want float64) {
	t.Helper()
	const eps = 1e-9
	if math.Abs(got-want) > eps {
		t.Fatalf("%s: got %v, want %v", name, got, want)
	}
}

func assertString(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %q, want %q", name, got, want)
	}
}

// fakeMarketRepo implements ordersMarketRepo (GetMarket) and is shared by the
// orders and intervals services.
type fakeMarketRepo struct {
	market *entity.Market
	err    error
	calls  []string
}

func (f *fakeMarketRepo) GetMarket(marketId string) (*entity.Market, error) {
	f.calls = append(f.calls, marketId)

	return f.market, f.err
}
