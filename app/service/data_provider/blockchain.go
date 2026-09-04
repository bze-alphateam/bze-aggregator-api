package data_provider

import (
	"context"
	"fmt"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/internal"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/sirupsen/logrus"
)

const (
	blockCacheDuration = 10 * time.Minute
	blockTimeKey       = "block_time:%d"
)

type blockCache interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte, expiration time.Duration) error
}

// blockClient is the slice of the CometBFT RPC client this provider needs.
// *http.HTTP satisfies it; declaring it here is what lets the block time
// caching be tested without a node.
type blockClient interface {
	Status(ctx context.Context) (*coretypes.ResultStatus, error)
	Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error)
	BlockResults(ctx context.Context, height *int64) (*coretypes.ResultBlockResults, error)
}

type BlockchainProvider struct {
	client blockClient
	cache  blockCache
	logger logrus.FieldLogger
}

func NewBlockchainProvider(client blockClient, cache blockCache, l logrus.FieldLogger) (*BlockchainProvider, error) {
	if client == nil || cache == nil || l == nil {
		return nil, internal.NewInvalidDependenciesErr("NewBlockchainProvider")
	}

	return &BlockchainProvider{
		client: client,
		cache:  cache,
		logger: l.WithField("data_provider", "BlockchainProvider"),
	}, nil
}

func (b BlockchainProvider) GetStatus() (*coretypes.ResultStatus, error) {
	return b.client.Status(context.Background())
}

func (b BlockchainProvider) GetBlock(height int64) (*coretypes.ResultBlock, error) {
	return b.client.Block(context.Background(), &height)
}

// GetBlockResults returns the ABCI results of a block: it is the only way to
// read events of already indexed blocks without asking the node to search by
// event type (tx_search), which overloads archive nodes.
func (b BlockchainProvider) GetBlockResults(ctx context.Context, height int64) (*coretypes.ResultBlockResults, error) {
	return b.client.BlockResults(ctx, &height)
}

// GetBlockTime retrieves the time of a block at the specified height using cache or blockchain data.
func (b BlockchainProvider) GetBlockTime(height int64) (time.Time, error) {
	cacheKey := fmt.Sprintf(blockTimeKey, height)
	cached, err := b.cache.Get(cacheKey)
	if err != nil {
		b.logger.WithError(err).Error("error getting cached block time")
	} else if cached != nil {
		return time.Parse(time.RFC3339Nano, string(cached))
	}

	block, err := b.GetBlock(height)
	if err != nil {
		return time.Time{}, err
	}

	// RFC3339Nano and not RFC3339: the latter has no fractional second, so it
	// would round-trip the block time through the cache with its milliseconds
	// stripped and callers would see a different timestamp for the same block
	// depending on whether they hit the cache.
	err = b.cache.Set(cacheKey, []byte(block.Block.Header.Time.Format(time.RFC3339Nano)), blockCacheDuration)
	if err != nil {
		b.logger.WithError(err).Error("error caching block time")
	}

	return block.Block.Header.Time, nil
}
