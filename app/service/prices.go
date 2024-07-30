package service

import (
	"encoding/json"
	"errors"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	pricesCache = "prices:all"

	priceCacheExpireSeconds = 180
)

type PriceProvider interface {
	GetDenominationsPrices() ([]dto.CoinPrice, error)
}

type PricesService struct {
	cache        Cache
	dataProvider PriceProvider
	logger       logrus.FieldLogger
}

func NewPricesService(cache Cache, dataProvider PriceProvider, logger logrus.FieldLogger) (*PricesService, error) {
	if dataProvider == nil || cache == nil || logger == nil {
		return nil, errors.New("invalid dependencies provided to price service")
	}

	return &PricesService{
		cache:        cache,
		dataProvider: dataProvider,
		logger:       logger,
	}, nil
}

func (p *PricesService) GetPrices() []dto.CoinPrice {
	cacheValue, err := p.cache.Get(pricesCache)
	if err != nil {
		p.logger.Errorf("failed to get prices from cache: %v", err)
	}

	if cacheValue != nil {
		var prices []dto.CoinPrice
		err = json.Unmarshal(cacheValue, &prices)
		if err != nil {
			p.logger.Errorf("failed to unmarshal prices from cache: %v", err)
		} else {
			return prices
		}
	}

	prices, err := p.dataProvider.GetDenominationsPrices()
	encoded, err := json.Marshal(prices)
	if err != nil {
		p.logger.Errorf("failed to marshal prices in order to cache them: %v", err)

		return prices
	}

	err = p.cache.Set(pricesCache, encoded, time.Duration(priceCacheExpireSeconds)*time.Second)
	if err != nil {
		p.logger.Errorf("failed to cache prices: %v", err)
	}

	return prices
}
