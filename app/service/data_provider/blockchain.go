package data_provider

import (
	"context"
	"fmt"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/cometbft/cometbft/rpc/client/http"
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

type BlockchainProvider struct {
	client *http.HTTP
	cache  blockCache
	logger logrus.FieldLogger
}

func NewBlockchainProvider(client *http.HTTP, cache blockCache, l logrus.FieldLogger) (*BlockchainProvider, error) {
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

// GetBlockTime retrieves the time of a block at the specified height using cache or blockchain data.
func (b BlockchainProvider) GetBlockTime(height int64) (time.Time, error) {
	cacheKey := fmt.Sprintf(blockTimeKey, height)
	cached, err := b.cache.Get(cacheKey)
	if err != nil {
		b.logger.WithError(err).Error("error getting cached block time")
	} else if cached != nil {
		return time.Parse(time.RFC3339, string(cached))
	}

	block, err := b.GetBlock(height)
	if err != nil {
		return time.Time{}, err
	}

	err = b.cache.Set(cacheKey, []byte(block.Block.Header.Time.Format(time.RFC3339)), blockCacheDuration)
	if err != nil {
		b.logger.WithError(err).Error("error caching block time")
	}

	return block.Block.Header.Time, nil
}
