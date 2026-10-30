package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/sirupsen/logrus"
)

const (
	marketHealthCacheKey   = "health:mh"
	marketHealthTtlMinutes = 10

	nodesAllowedHeightDiff = 2
)

type MarketHistoryProvider interface {
	GetMarketHistory(marketId string, limit int) ([]dto.HistoryOrder, error)
}

// NodeInfoClient reports a node's status (the CometBFT RPC client satisfies it).
type NodeInfoClient interface {
	Status(ctx context.Context) (*coretypes.ResultStatus, error)
}

type Cache interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte, expiration time.Duration) error
}

type Health struct {
	logger logrus.FieldLogger
	cache  Cache

	provider MarketHistoryProvider

	nodesPool map[string]NodeInfoClient
}

func NewHealthService(logger logrus.FieldLogger, cache Cache, provider MarketHistoryProvider, nodes map[string]NodeInfoClient) (*Health, error) {
	if logger == nil || cache == nil || provider == nil {
		return nil, internal.NewInvalidDependenciesErr("NewHealthService")
	}

	return &Health{
		logger:    logger.WithField("service", "Service.Health"),
		cache:     cache,
		provider:  provider,
		nodesPool: nodes,
	}, nil
}

func (h *Health) GetMarketHealth(marketId string, minutesAgo int) dto.MarketHealth {
	cached := h.getCachedMarketHealth(marketId, minutesAgo)
	if cached != nil {
		return *cached
	}

	var mh dto.MarketHealth
	marketHist, err := h.provider.GetMarketHistory(marketId, 1)
	if err != nil {
		h.logger.WithError(err).Error("error getting market history")

		return mh
	}

	if len(marketHist) == 0 {

		return mh
	}

	currentTime := time.Now().UTC()
	// Subtract the minutes by creating a negative duration
	minDateNeeded := currentTime.Add(-time.Duration(minutesAgo) * time.Minute)

	mh.IsHealthy = minDateNeeded.Before(marketHist[0].ExecutedAt)
	mh.LastTrade = marketHist[0].ExecutedAt

	toCache, err := json.Marshal(mh)
	if err != nil {
		h.logger.WithError(err).Error("error marshalling market health")

		return mh
	}

	err = h.cache.Set(h.getMarketHealthCacheKey(marketId), toCache, marketHealthTtlMinutes*time.Minute)
	if err != nil {
		h.logger.WithError(err).Error("error caching market health")
	}

	return mh
}

func (h *Health) getCachedMarketHealth(marketId string, minutesAgo int) *dto.MarketHealth {
	cached, err := h.cache.Get(h.getMarketHealthCacheKey(marketId))
	if err != nil {
		h.logger.WithError(err).Error("error getting cached market health")

		return nil
	}

	if cached == nil {
		return nil
	}

	var mh dto.MarketHealth
	err = json.Unmarshal(cached, &mh)
	if err != nil {
		h.logger.WithError(err).Error("error unmarshalling cached market health")

		return nil
	}

	currentTime := time.Now().UTC()
	// Subtract the minutes by creating a negative duration
	minDateNeeded := currentTime.Add(-time.Duration(minutesAgo) * time.Minute)

	if minDateNeeded.Before(mh.LastTrade) {
		mh.IsHealthy = true
		return &mh
	}

	return nil
}

func (h *Health) getMarketHealthCacheKey(marketId string) string {
	return fmt.Sprintf("%s:%s", marketHealthCacheKey, marketId)
}

func (h *Health) GetNodesHealth() dto.NodesHealth {
	if len(h.nodesPool) == 0 {
		return dto.NodesHealth{}
	}
	l := h.logger.WithField("func", "GetNodesHealth")

	resp := make(map[string]*coretypes.ResultStatus, len(h.nodesPool))
	var wg sync.WaitGroup
	var mx sync.Mutex
	var errorsStr string
	for name, infoClient := range h.nodesPool {
		wg.Add(1)
		go func() {
			defer wg.Done()
			info, err := infoClient.Status(context.Background())
			mx.Lock()
			defer mx.Unlock()
			if err != nil {
				errorsStr += fmt.Sprintf("failed to query: %s;", name)
				l.WithError(err).WithField("name", name).Error("failed to get latest block")

				return
			}

			resp[name] = info
		}()
	}

	wg.Wait()

	for name, info := range resp {
		if info == nil {
			errorsStr += fmt.Sprintf("no info found for: %s;", name)
			continue
		}

		for name2, info2 := range resp {
			if info2 == nil || name == name2 {
				continue
			}

			diff := info.SyncInfo.LatestBlockHeight - info2.SyncInfo.LatestBlockHeight
			if diff > nodesAllowedHeightDiff {
				errorsStr += fmt.Sprintf("%s is behind %s. Expected height %d, found %d", name2, name, info.SyncInfo.LatestBlockHeight, info2.SyncInfo.LatestBlockHeight)
			}
		}
	}

	return dto.NodesHealth{
		IsHealthy: errorsStr == "",
		Errors:    errorsStr,
	}
}
