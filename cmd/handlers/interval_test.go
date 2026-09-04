package handlers

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
)

const (
	// a real pool id: pools are "base_quote", and their base can itself hold
	// slashes, which is exactly why they cannot be matched as market ids
	poolId   = "factory/bze13gzq40che93tgfm9kzmkpjamah5nj0j73pyhqk/uvdl_ubze"
	marketId = "factory/bze13gzq40che93tgfm9kzmkpjamah5nj0j73pyhqk/uvdl/ubze"
)

type fakeMarketProvider struct {
	markets []types.Market
	err     error
}

func (p *fakeMarketProvider) GetAllMarkets() ([]types.Market, error) {
	return p.markets, p.err
}

type fakeLpProvider struct {
	pools []string
	err   error
	calls int
}

func (p *fakeLpProvider) GetAllLiquidityPoolsIds() ([]string, error) {
	p.calls++

	return p.pools, p.err
}

type fakeIntervalStorage struct {
	synced []string
	err    error
}

func (s *fakeIntervalStorage) SyncIntervals(marketId string) error {
	s.synced = append(s.synced, marketId)

	return s.err
}

func newTestIntervalSync(t *testing.T) (*MarketIntervalSync, *fakeMarketProvider, *fakeLpProvider, *fakeIntervalStorage) {
	t.Helper()

	mProvider := &fakeMarketProvider{markets: []types.Market{
		{Base: "factory/bze13gzq40che93tgfm9kzmkpjamah5nj0j73pyhqk/uvdl", Quote: "ubze"},
		{Base: "utest", Quote: "ubze"},
	}}
	lpProvider := &fakeLpProvider{pools: []string{poolId, "ibc/ABCD_ubze"}}
	storage := &fakeIntervalStorage{}

	logger := logrus.New()
	logger.SetOutput(&strings.Builder{})

	handler, err := NewMarketIntervalSync(logger, mProvider, storage, lpProvider)
	if err != nil {
		t.Fatalf("could not build the handler: %v", err)
	}

	return handler, mProvider, lpProvider, storage
}

// TestSyncIntervalsForALiquidityPool is the regression test: a pool id used to
// be looked for among the chain's markets only, where it can never match.
func TestSyncIntervalsForALiquidityPool(t *testing.T) {
	handler, _, _, storage := newTestIntervalSync(t)

	if err := handler.SyncIntervals(poolId); err != nil {
		t.Fatalf("syncing a pool failed: %v", err)
	}

	if len(storage.synced) != 1 || storage.synced[0] != poolId {
		t.Fatalf("expected the pool id to reach the interval sync, got %v", storage.synced)
	}
}

// TestSyncIntervalsForAMarketStillWorks guards the behaviour the command had
// before pools were accepted.
func TestSyncIntervalsForAMarketStillWorks(t *testing.T) {
	handler, _, _, storage := newTestIntervalSync(t)

	if err := handler.SyncIntervals(marketId); err != nil {
		t.Fatalf("syncing a market failed: %v", err)
	}

	if len(storage.synced) != 1 || storage.synced[0] != marketId {
		t.Fatalf("expected the market id to reach the interval sync, got %v", storage.synced)
	}
}

func TestSyncIntervalsRejectsAnUnknownId(t *testing.T) {
	handler, _, _, storage := newTestIntervalSync(t)

	err := handler.SyncIntervals("nope/nothing")
	if err == nil {
		t.Fatal("expected an unknown id to be rejected")
	}

	if !strings.Contains(err.Error(), "nope/nothing") {
		t.Fatalf("expected the error to name the id, got %q", err)
	}

	if len(storage.synced) != 0 {
		t.Fatalf("expected nothing to be synced, got %v", storage.synced)
	}
}

// TestSyncIntervalsDoesNotAskTheChainForAPool keeps the pool path off the chain
// query: an operator rebuilding pool candles should not need the markets call
// to succeed.
func TestSyncIntervalsDoesNotAskTheChainForAPool(t *testing.T) {
	handler, mProvider, _, storage := newTestIntervalSync(t)
	mProvider.err = errors.New("node is down")
	mProvider.markets = nil

	if err := handler.SyncIntervals(poolId); err != nil {
		t.Fatalf("syncing a pool failed: %v", err)
	}

	if len(storage.synced) != 1 || storage.synced[0] != poolId {
		t.Fatalf("expected the pool id to reach the interval sync, got %v", storage.synced)
	}
}

func TestSyncIntervalsFailsWhenThePoolsCannotBeRead(t *testing.T) {
	handler, _, lpProvider, storage := newTestIntervalSync(t)
	lpProvider.err = errors.New("database is down")
	lpProvider.pools = nil

	if err := handler.SyncIntervals(poolId); err == nil {
		t.Fatal("expected the pool lookup failure to be reported")
	}

	if len(storage.synced) != 0 {
		t.Fatalf("expected nothing to be synced, got %v", storage.synced)
	}
}

// TestSyncAllCoversMarketsAndPools is the second half of the fix: the
// no-argument form never visited the pools.
func TestSyncAllCoversMarketsAndPools(t *testing.T) {
	handler, _, _, storage := newTestIntervalSync(t)

	handler.SyncAll()

	want := []string{
		marketId,
		"utest/ubze",
		poolId,
		"ibc/ABCD_ubze",
	}

	for _, id := range want {
		if !slices.Contains(storage.synced, id) {
			t.Fatalf("expected %s to be synced, got %v", id, storage.synced)
		}
	}

	if len(storage.synced) != len(want) {
		t.Fatalf("expected exactly %d syncs, got %v", len(want), storage.synced)
	}
}

// TestSyncAllKeepsGoingWhenAPoolFails - one broken pool must not hide the rest,
// which is how syncAll already behaves for markets.
func TestSyncAllKeepsGoingWhenAPoolFails(t *testing.T) {
	handler, mProvider, _, storage := newTestIntervalSync(t)
	mProvider.markets = nil
	storage.err = errors.New("no orders found to add to intervals")

	handler.SyncAll()

	if len(storage.synced) != 2 {
		t.Fatalf("expected both pools to be attempted, got %v", storage.synced)
	}
}

func TestSyncAllSurvivesAPoolLookupFailure(t *testing.T) {
	handler, _, lpProvider, storage := newTestIntervalSync(t)
	lpProvider.err = errors.New("database is down")
	lpProvider.pools = nil

	handler.SyncAll()

	// the markets are synced before the pools are read, so they still land
	if len(storage.synced) != 2 {
		t.Fatalf("expected the markets to still be synced, got %v", storage.synced)
	}
}

func TestNewMarketIntervalSyncRequiresItsDependencies(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&strings.Builder{})

	if _, err := NewMarketIntervalSync(logger, &fakeMarketProvider{}, &fakeIntervalStorage{}, nil); err == nil {
		t.Fatal("expected a missing liquidity pool provider to be rejected")
	}
}
