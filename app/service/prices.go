package service

import (
	"encoding/json"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	pricesCache       = "prices:all"
	pricesCacheBackup = "prices:all:backup"

	priceCacheExpireSeconds       = 180
	priceBackupCacheExpireSeconds = 60 * 60 * 24 //1 day
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
		return nil, internal.NewInvalidDependenciesErr("NewPricesService")
	}

	return &PricesService{
		cache:        cache,
		dataProvider: dataProvider,
		logger:       logger,
	}, nil
}

func (p *PricesService) GetPrices() []dto.CoinPrice {
	cacheValue := p.getPricesFromCache(pricesCache)
	if cacheValue != nil {
		return cacheValue
	}

	prices := p.getPricesFromProvider()
	if prices == nil {
		//return backup
		return p.getPricesFromCache(pricesCacheBackup)
	}

	p.cachePrices(prices)

	return prices
}

func (p *PricesService) cachePrices(prices []dto.CoinPrice) {
	encoded, err := json.Marshal(prices)
	if err != nil {
		p.logger.Errorf("failed to marshal prices in order to cache them: %v", err)

		return
	}

	err = p.cache.Set(pricesCache, encoded, time.Duration(priceCacheExpireSeconds)*time.Second)
	if err != nil {
		p.logger.Errorf("failed to cache prices: %v", err)
	}

	err = p.cache.Set(pricesCacheBackup, encoded, time.Duration(priceBackupCacheExpireSeconds)*time.Second)
	if err != nil {
		p.logger.Errorf("failed to cache prices for backup: %v", err)
	}
}

func (p *PricesService) getBackupPrices() []dto.CoinPrice {
	return p.getPricesFromCache(pricesCacheBackup)
}

func (p *PricesService) getPricesFromCache(key string) []dto.CoinPrice {
	cacheValue, err := p.cache.Get(key)
	if err != nil {
		p.logger.Errorf("failed to get prices from cache: %v", err)

		return nil
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

	return nil
}

func (p *PricesService) getPricesFromProvider() []dto.CoinPrice {
	prices, err := p.dataProvider.GetDenominationsPrices()
	if err != nil {
		p.logger.Errorf("failed to get prices from provider: %v", err)

		return nil
	}

	return prices
}
