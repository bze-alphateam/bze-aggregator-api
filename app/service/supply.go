package service

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
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
)

type chainRegistry interface {
	GetAssetDetails(denom string) (*chain_registry.ChainRegistryAsset, error)
}

type RestDataProvider interface {
	GetTotalSupply(denom string) (int64, error)
	GetCommunityPoolTotal(denom string) (float64, error)
}

type Cache interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte, expiration time.Duration) error
}

type Supply struct {
	cache        Cache
	dataProvider RestDataProvider
	logger       logrus.FieldLogger
	registry     chainRegistry
}

func NewSupplyService(logger logrus.FieldLogger, cache Cache, provider RestDataProvider, registry chainRegistry) (*Supply, error) {
	if logger == nil || cache == nil || provider == nil || registry == nil {
		return nil, internal.NewInvalidDependenciesErr("NewSupplyService")
	}

	return &Supply{
		cache:        cache,
		dataProvider: provider,
		logger:       logger.WithField("service", "Service.Supply"),
		registry:     registry,
	}, nil
}

func (s *Supply) GetTotalSupply(denom string) (string, error) {
	display, err := s.getDisplayDenom(denom)
	if err != nil {
		return "", err
	}

	cacheKey := s.getTotalSupplyCacheKey(denom)
	cacheValue, err := s.cache.Get(cacheKey)
	if err != nil {
		s.logger.Errorf("failed to get total supply from cache: %v", err)
	}

	if cacheValue != nil {
		return string(cacheValue), nil
	}

	uTotalSupply, err := s.dataProvider.GetTotalSupply(denom)
	if err != nil {
		s.logger.Errorf("failed to get total supply from data provider: %v", err)

		return "0", nil
	}

	totalSupply := float64(uTotalSupply) / math.Pow(10, float64(display.Exponent))
	supplyStr := fmt.Sprintf("%.2f", totalSupply)

	err = s.cache.Set(cacheKey, []byte(supplyStr), time.Duration(cacheExpireSeconds)*time.Second)
	if err != nil {
		s.logger.Errorf("failed to set total supply to cache: %v", err)
	}

	return supplyStr, nil
}

func (s *Supply) GetCirculatingSupply(denom string) (string, error) {
	display, err := s.getDisplayDenom(denom)
	if err != nil {
		return "", err
	}

	if !display.IsBZE() {
		return s.GetTotalSupply(denom)
	}

	cacheKey := s.getCirculatingSupplyCacheKey(denom)
	cacheValue, err := s.cache.Get(cacheKey)
	if err != nil {
		s.logger.Errorf("failed to get circulating supply from cache: %v", err)
	}

	if cacheValue != nil {
		return string(cacheValue), nil
	}

	// Get the total supply as string
	totalSupplyStr, _ := s.GetTotalSupply(denom)

	// Convert total supply to float64
	totalSupplyFloat, _ := strconv.ParseFloat(totalSupplyStr, 64)

	// Get the community pool total
	communityPoolTotal, err := s.dataProvider.GetCommunityPoolTotal(denom)
	if err != nil {
		s.logger.Errorf("failed to get community pool funds: %v", err)

		return "0", nil
	}

	adjustedPoolTotal := communityPoolTotal / math.Pow(10, float64(display.Exponent))

	// Calculate the adjusted supply
	adjustedSupply := totalSupplyFloat - adjustedPoolTotal
	resultStr := fmt.Sprintf("%.2f", adjustedSupply)
	err = s.cache.Set(cacheKey, []byte(resultStr), time.Duration(cacheExpireSeconds)*time.Second)
	if err != nil {
		s.logger.Errorf("failed to set circulating supply to cache: %v", err)
	}

	return resultStr, nil
}

func (s *Supply) getTotalSupplyCacheKey(denom string) string {
	return fmt.Sprintf("%s:%s", totalSupplyCacheKey, denom)
}

func (s *Supply) getCirculatingSupplyCacheKey(denom string) string {
	return fmt.Sprintf("%s:%s", circulatingSupplyCacheKey, denom)
}

func (s *Supply) getDisplayDenom(denom string) (*chain_registry.ChainRegistryAssetDenom, error) {
	details, err := s.registry.GetAssetDetails(denom)
	if err != nil {
		return nil, fmt.Errorf("denom %s not found in registry", denom)
	}

	display := details.GetDisplayDenomUnit()
	if display == nil {
		return nil, fmt.Errorf("%s has no display denomination", denom)
	}

	return display, nil
}

// GetUTotalSupply returns the total supply in micro amounts (raw value from blockchain)
func (s *Supply) GetUTotalSupply(denom string) (string, error) {
	uTotalSupply, err := s.dataProvider.GetTotalSupply(denom)
	if err != nil {
		s.logger.Errorf("failed to get total supply from data provider: %v", err)
		return "0", err
	}

	return fmt.Sprintf("%d", uTotalSupply), nil
}
