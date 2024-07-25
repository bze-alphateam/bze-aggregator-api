package service

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"math"
	"time"
)

const (
	totalSupplyCacheKey       = "supply:total_supply"
	circulatingSupplyCacheKey = "supply:circulating_supply"

	cacheExpireSeconds = 15

	decimals = 6
)

type RestDataProvider interface {
	GetTotalSupply() (int64, error)
}

type Cache interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte, expiration time.Duration) error
}

type Supply struct {
	cache        Cache
	dataProvider RestDataProvider
	logger       logrus.FieldLogger
}

func NewSupplyService(logger logrus.FieldLogger, cache Cache, provider RestDataProvider) (*Supply, error) {
	if logger == nil || cache == nil || provider == nil {
		return nil, errors.New("invalid dependencies provided to supply service")
	}

	return &Supply{
		cache:        cache,
		dataProvider: provider,
		logger:       logger,
	}, nil
}

func (s *Supply) GetTotalSupply() string {
	cacheValue, err := s.cache.Get(totalSupplyCacheKey)
	if err != nil {
		s.logger.Errorf("failed to get total supply from cache: %v", err)
	}

	if cacheValue != nil {
		return string(cacheValue)
	}

	uTotalSupply, err := s.dataProvider.GetTotalSupply()
	if err != nil {
		s.logger.Errorf("failed to get total supply from data provider: %v", err)

		return "0"
	}

	totalSupply := float64(uTotalSupply) / math.Pow(10, float64(decimals))
	supplyStr := fmt.Sprintf("%.2f", totalSupply)

	err = s.cache.Set(totalSupplyCacheKey, []byte(supplyStr), time.Duration(cacheExpireSeconds)*time.Second)
	if err != nil {
		s.logger.Errorf("failed to set total supply to cacheL %v", err)
	}

	return supplyStr
}

func (s *Supply) GetCirculatingSupply() string {
	return "0"
	//cacheValue, err := s.cache.Get(totalSupplyCacheKey)
	//if err != nil {
	//	s.logger.Errorf("failed to get total supply from cacheL %v", err)
	//}
	//
	//if cacheValue != nil {
	//	return string(cacheValue)
	//}
	//
	//uTotalSupply, err := s.dataProvider.GetTotalSupply()
	//if err != nil {
	//	s.logger.Errorf("failed to get total supply from data provider")
	//
	//	return "0"
	//}
	//
	//totalSupply := float64(uTotalSupply) / math.Pow(10, float64(decimals))
	//supplyStr := fmt.Sprintf("%.2f", totalSupply)
	//
	//err = s.cache.Set(totalSupplyCacheKey, []byte(supplyStr))
	//if err != nil {
	//	s.logger.Errorf("failed to set total supply to cache: %v", err)
	//}
	//
	//return supplyStr
}
