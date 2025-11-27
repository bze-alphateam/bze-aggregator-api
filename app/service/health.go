package service

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/request"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/sirupsen/logrus"
)

const (
	marketHealthCacheKey   = "health:mh"
	aggHealthCacheKey      = "health:agg"
	marketHealthTtlMinutes = 10

	nodesAllowedHeightDiff = 2
)

type MarketHistoryProvider interface {
	GetMarketHistory(marketId string, limit int) ([]dto.HistoryOrder, error)
}

type NodeInfoClient interface {
	GetStatus() (*coretypes.ResultStatus, error)
}

type internalHistoryProvider interface {
	GetHistoryBy(params request.HistoryParams) ([]entity.MarketHistory, error)
}

type Health struct {
	logger logrus.FieldLogger
	cache  Cache

	provider                MarketHistoryProvider
	internalHistoryProvider internalHistoryProvider

	nodesPool map[string]NodeInfoClient
}

func NewHealthService(logger logrus.FieldLogger, cache Cache, provider MarketHistoryProvider, internalHistoryProvider internalHistoryProvider, nodes map[string]NodeInfoClient) (*Health, error) {
	if logger == nil || cache == nil || provider == nil || internalHistoryProvider == nil {
		return nil, internal.NewInvalidDependenciesErr("NewHealthService")
	}

	return &Health{
		logger:                  logger.WithField("service", "Service.Health"),
		cache:                   cache,
		provider:                provider,
		internalHistoryProvider: internalHistoryProvider,
		nodesPool:               nodes,
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

func (h *Health) getAggregatorHealthCacheKey() string {
	return aggHealthCacheKey
}

func (h *Health) GetAggregatorHealth(minutesAgo int) dto.AggregatorHealth {
	cached := h.getCachedAggregatorHealth(minutesAgo)
	if cached != nil {
		return *cached
	}
	currentTime := time.Now().UTC()
	minDateNeeded := currentTime.Add(-time.Duration(minutesAgo) * time.Minute)

	histParams := request.HistoryParams{
		Limit:     1,
		StartTime: minDateNeeded.UnixMilli(),
		EndTime:   currentTime.UnixMilli(),
	}
	marketHist, err := h.internalHistoryProvider.GetHistoryBy(histParams)

	var mh dto.AggregatorHealth
	if err != nil {
		h.logger.WithError(err).Error("error getting internal market history")

		return mh
	}

	if len(marketHist) == 0 {

		return mh
	}

	mh.IsHealthy = minDateNeeded.Before(marketHist[0].ExecutedAt)
	mh.LastSync = marketHist[0].CreatedAt

	toCache, err := json.Marshal(mh)
	if err != nil {
		h.logger.WithError(err).Error("error marshalling market health")

		return mh
	}

	err = h.cache.Set(h.getAggregatorHealthCacheKey(), toCache, marketHealthTtlMinutes*time.Minute)
	if err != nil {
		h.logger.WithError(err).Error("error caching market health")
	}

	return mh
}

func (h *Health) getCachedAggregatorHealth(minutesAgo int) *dto.AggregatorHealth {
	cached, err := h.cache.Get(h.getAggregatorHealthCacheKey())
	if err != nil {
		h.logger.WithError(err).Error("error getting cached aggregator health")

		return nil
	}

	if cached == nil {
		return nil
	}

	var ah dto.AggregatorHealth
	err = json.Unmarshal(cached, &ah)
	if err != nil {
		h.logger.WithError(err).Error("error unmarshalling cached aggregator health")

		return nil
	}

	currentTime := time.Now().UTC()
	// Subtract the minutes by creating a negative duration
	minDateNeeded := currentTime.Add(-time.Duration(minutesAgo) * time.Minute)

	if minDateNeeded.Before(ah.LastSync) {
		ah.IsHealthy = true
		return &ah
	}

	return nil
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
			info, err := infoClient.GetStatus()
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
