package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	marketHealthCacheKey   = "health:mh"
	marketHealthTtlMinutes = 10
)

type MarketHistoryProvider interface {
	GetMarketHistory(marketId string, limit int) ([]dto.HistoryOrder, error)
}

type Health struct {
	logger logrus.FieldLogger
	cache  Cache

	provider MarketHistoryProvider
}

func NewHealthService(logger logrus.FieldLogger, cache Cache, provider MarketHistoryProvider) (*Health, error) {
	if logger == nil || cache == nil || provider == nil {
		return nil, errors.New("invalid dependencies provided to NewHealthService")
	}

	return &Health{
		logger:   logger,
		cache:    cache,
		provider: provider,
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
		return &mh
	}

	return nil
}

func (h *Health) getMarketHealthCacheKey(marketId string) string {
	return fmt.Sprintf("%s:%s", marketHealthCacheKey, marketId)
}
