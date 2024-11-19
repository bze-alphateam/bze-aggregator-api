package data_provider

import (
	"encoding/json"
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	cacheTtl = 30 * time.Minute
)

type registryCache interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte, expiration time.Duration) error
}

type registryStore interface {
	GetAssetList() (*chain_registry.ChainRegistryAssetList, error)
}

type ChainRegistry struct {
	cache  registryCache
	store  registryStore
	logger logrus.FieldLogger
}

func NewChainRegistry(logger logrus.FieldLogger, cache registryCache, store registryStore) (*ChainRegistry, error) {
	if cache == nil || store == nil || logger == nil {
		return nil, internal.NewInvalidDependenciesErr("NewChainRegistry")
	}

	return &ChainRegistry{
		cache:  cache,
		store:  store,
		logger: logger.WithField("service", "DataProvider.ChainRegistry"),
	}, nil
}

func (r *ChainRegistry) GetAssetDetails(denom string) (*chain_registry.ChainRegistryAsset, error) {
	l := r.logger.WithField("denom", denom)
	cached, err := r.getAssetDetailsFromCache(denom)
	if err != nil {
		l.Errorf("Error getting asset details from cache: %v", err)
	}

	if cached != nil {
		return cached, nil
	}

	assetsList, err := r.store.GetAssetList()
	if err != nil {
		return nil, err
	}

	if assetsList == nil {
		return nil, fmt.Errorf("no chain registry assets found")
	}

	for _, a := range assetsList.Assets {
		data, err := json.Marshal(a)
		if err != nil {
			l.WithError(err).Error("error marshalling asset to cache")
			continue
		}

		err = r.cache.Set(a.Base, data, cacheTtl)
		if err != nil {
			l.WithError(err).Error("error caching asset")
			continue
		}
	}

	return r.getAssetDetailsFromCache(denom)
}

func (r *ChainRegistry) getAssetDetailsFromCache(denom string) (*chain_registry.ChainRegistryAsset, error) {
	l := r.logger.WithField("denom", denom)
	cache, err := r.cache.Get(denom)
	if err != nil {
		l.WithError(err).Warn("error when trying to get chain registry asset details from cache")
	}

	if len(cache) > 0 {
		resp := &chain_registry.ChainRegistryAsset{}
		err := json.Unmarshal(cache, resp)
		if err == nil {
			return resp, nil
		}

		l.WithError(err).Warn("error when trying to unmarshal chain registry asset details from cache")
	}

	return nil, nil
}
