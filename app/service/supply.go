package service

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
	"math"
	"strconv"
	"time"
)

const (
	totalSupplyCacheKey       = "supply:total_supply"
	circulatingSupplyCacheKey = "supply:circulating_supply"

	cacheExpireSeconds = 600

	decimals = 6
)

type RestDataProvider interface {
	GetTotalSupply() (int64, error)
	GetCommunityPoolTotal() (float64, error)
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
		return nil, internal.NewInvalidDependenciesErr("NewSupplyService")
	}

	return &Supply{
		cache:        cache,
		dataProvider: provider,
		logger:       logger.WithField("service", "Service.Supply"),
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
		s.logger.Errorf("failed to set total supply to cache: %v", err)
	}

	return supplyStr
}

func (s *Supply) GetCirculatingSupply() string {
	cacheValue, err := s.cache.Get(circulatingSupplyCacheKey)
	if err != nil {
		s.logger.Errorf("failed to get circulating supply from cache: %v", err)
	}

	if cacheValue != nil {
		return string(cacheValue)
	}

	// Get the total supply as string
	totalSupplyStr := s.GetTotalSupply()

	// Convert total supply to float64
	totalSupplyFloat, _ := strconv.ParseFloat(totalSupplyStr, 64)

	// Get the community pool total
	communityPoolTotal, err := s.dataProvider.GetCommunityPoolTotal()
	if err != nil {
		s.logger.Errorf("failed to get community pool funds: %v", err)

		return "0"
	}

	adjustedPoolTotal := communityPoolTotal / math.Pow(10, float64(decimals))

	// Calculate the adjusted supply
	adjustedSupply := totalSupplyFloat - adjustedPoolTotal
	resultStr := fmt.Sprintf("%.2f", adjustedSupply)
	err = s.cache.Set(circulatingSupplyCacheKey, []byte(resultStr), time.Duration(cacheExpireSeconds)*time.Second)
	if err != nil {
		s.logger.Errorf("failed to set circulating supply to cache: %v", err)
	}

	return resultStr
}
