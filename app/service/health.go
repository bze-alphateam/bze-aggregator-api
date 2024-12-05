package service

import (
	"encoding/json"
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/request"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	marketHealthCacheKey   = "health:mh"
	aggHealthCacheKey      = "health:agg"
	marketHealthTtlMinutes = 10
)

type MarketHistoryProvider interface {
	GetMarketHistory(marketId string, limit int) ([]dto.HistoryOrder, error)
}

type internalHistoryProvider interface {
	GetHistoryBy(params request.HistoryParams) ([]entity.MarketHistory, error)
}

type Health struct {
	logger logrus.FieldLogger
	cache  Cache

	provider                MarketHistoryProvider
	internalHistoryProvider internalHistoryProvider
}

func NewHealthService(logger logrus.FieldLogger, cache Cache, provider MarketHistoryProvider, internalHistoryProvider internalHistoryProvider) (*Health, error) {
	if logger == nil || cache == nil || provider == nil || internalHistoryProvider == nil {
		return nil, internal.NewInvalidDependenciesErr("NewHealthService")
	}

	return &Health{
		logger:                  logger.WithField("service", "Service.Health"),
		cache:                   cache,
		provider:                provider,
		internalHistoryProvider: internalHistoryProvider,
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
