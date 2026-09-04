package data_provider

import (
	"context"
	"testing"
	"time"

	cometbfttypes "github.com/cometbft/cometbft/types"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/sirupsen/logrus"
)

// blockTime is a real mainnet block time: it has both milliseconds and
// nanoseconds, which is what makes it worth asserting on.
var blockTime = time.Date(2025, 12, 17, 23, 27, 8, 267901371, time.UTC)

type fakeCache struct {
	data map[string][]byte
	sets int
}

func newFakeCache() *fakeCache {
	return &fakeCache{data: make(map[string][]byte)}
}

func (c *fakeCache) Get(key string) ([]byte, error) {
	return c.data[key], nil
}

func (c *fakeCache) Set(key string, data []byte, _ time.Duration) error {
	c.data[key] = data
	c.sets++

	return nil
}

type fakeBlockClient struct {
	blockTime time.Time
	calls     int
}

func (c *fakeBlockClient) Status(context.Context) (*coretypes.ResultStatus, error) {
	return &coretypes.ResultStatus{}, nil
}

func (c *fakeBlockClient) Block(_ context.Context, _ *int64) (*coretypes.ResultBlock, error) {
	c.calls++

	return &coretypes.ResultBlock{
		Block: &cometbfttypes.Block{
			Header: cometbfttypes.Header{Time: c.blockTime},
		},
	}, nil
}

func (c *fakeBlockClient) BlockResults(context.Context, *int64) (*coretypes.ResultBlockResults, error) {
	return &coretypes.ResultBlockResults{}, nil
}

func newTestProvider(t *testing.T) (*BlockchainProvider, *fakeBlockClient, *fakeCache) {
	t.Helper()

	client := &fakeBlockClient{blockTime: blockTime}
	cache := newFakeCache()

	provider, err := NewBlockchainProvider(client, cache, logrus.New())
	if err != nil {
		t.Fatalf("could not build the provider: %v", err)
	}

	return provider, client, cache
}

// TestGetBlockTimeKeepsSubSecondPrecisionOnCacheHit is the regression test for
// the truncating cache: the second call is served from the cache and used to
// come back rounded down to the whole second.
func TestGetBlockTimeKeepsSubSecondPrecisionOnCacheHit(t *testing.T) {
	provider, client, cache := newTestProvider(t)

	first, err := provider.GetBlockTime(20546750)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	if client.calls != 1 {
		t.Fatalf("expected the first call to reach the node once, got %d calls", client.calls)
	}

	if cache.sets != 1 {
		t.Fatalf("expected the first call to cache the block time, got %d writes", cache.sets)
	}

	second, err := provider.GetBlockTime(20546750)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if client.calls != 1 {
		t.Fatalf("expected the second call to be served from the cache, got %d node calls", client.calls)
	}

	if !second.Equal(first) {
		t.Fatalf("cached block time %s does not match the fetched one %s", second.Format(time.RFC3339Nano), first.Format(time.RFC3339Nano))
	}

	if second.UnixNano() != blockTime.UnixNano() {
		t.Fatalf("expected the cached block time to keep its sub-second part, want %s got %s", blockTime.Format(time.RFC3339Nano), second.Format(time.RFC3339Nano))
	}

	// market_history.executed_at is DATETIME(3), so milliseconds are what
	// actually reaches the database - assert on them explicitly.
	if got, want := second.UnixMilli(), blockTime.UnixMilli(); got != want {
		t.Fatalf("expected the cached block time to keep its milliseconds, want %d got %d", want, got)
	}
}

// TestGetBlockTimeIsStableAcrossManyCacheHits covers the shape the bug actually
// showed up in: several swap events of the same block each ask for its time.
func TestGetBlockTimeIsStableAcrossManyCacheHits(t *testing.T) {
	provider, client, _ := newTestProvider(t)

	first, err := provider.GetBlockTime(20546750)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		got, err := provider.GetBlockTime(20546750)
		if err != nil {
			t.Fatalf("call %d failed: %v", i+2, err)
		}

		if !got.Equal(first) {
			t.Fatalf("call %d returned %s, want %s", i+2, got.Format(time.RFC3339Nano), first.Format(time.RFC3339Nano))
		}
	}

	if client.calls != 1 {
		t.Fatalf("expected a single node call for the whole block, got %d", client.calls)
	}
}

// TestGetBlockTimeOfASecondAlignedBlock guards the RFC3339Nano round-trip for a
// timestamp whose fractional part is zero: the layout trims it away entirely,
// and the parse side has to cope with that.
func TestGetBlockTimeOfASecondAlignedBlock(t *testing.T) {
	aligned := time.Date(2025, 12, 17, 23, 27, 8, 0, time.UTC)

	client := &fakeBlockClient{blockTime: aligned}
	provider, err := NewBlockchainProvider(client, newFakeCache(), logrus.New())
	if err != nil {
		t.Fatalf("could not build the provider: %v", err)
	}

	if _, err = provider.GetBlockTime(1); err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	cached, err := provider.GetBlockTime(1)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if !cached.Equal(aligned) {
		t.Fatalf("cached block time %s does not match %s", cached.Format(time.RFC3339Nano), aligned.Format(time.RFC3339Nano))
	}
}

// TestGetBlockTimeCachesPerHeight makes sure the cache key is not shared between
// blocks - the two swaps of the reproduction live in different blocks as often
// as they live in the same one.
func TestGetBlockTimeCachesPerHeight(t *testing.T) {
	provider, client, cache := newTestProvider(t)

	if _, err := provider.GetBlockTime(20546750); err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	client.blockTime = blockTime.Add(6 * time.Second)
	other, err := provider.GetBlockTime(20546751)
	if err != nil {
		t.Fatalf("second height failed: %v", err)
	}

	if !other.Equal(client.blockTime) {
		t.Fatalf("expected the second height to be fetched, got %s", other.Format(time.RFC3339Nano))
	}

	if len(cache.data) != 2 {
		t.Fatalf("expected one cache entry per height, got %d", len(cache.data))
	}
}
