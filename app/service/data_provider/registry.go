package data_provider

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/sirupsen/logrus"
)

const (
	cacheTtl = 30 * time.Minute
)

type metadataProvider interface {
	GetAllDenomsMetadata() ([]banktypes.Metadata, error)
}

type registryCache interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte, expiration time.Duration) error
}

type registryStore interface {
	GetAssetList() (*chain_registry.ChainRegistryAssetList, error)
}

type ChainRegistry struct {
	cache    registryCache
	store    registryStore
	logger   logrus.FieldLogger
	metadata metadataProvider
}

func NewChainRegistry(logger logrus.FieldLogger, cache registryCache, store registryStore, metadata metadataProvider) (*ChainRegistry, error) {
	if cache == nil || store == nil || logger == nil || metadata == nil {
		return nil, internal.NewInvalidDependenciesErr("NewChainRegistry")
	}

	return &ChainRegistry{
		cache:    cache,
		store:    store,
		logger:   logger.WithField("service", "DataProvider.ChainRegistry"),
		metadata: metadata,
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

	wg := sync.WaitGroup{}
	wg.Add(2)

	var assetsList *chain_registry.ChainRegistryAssetList
	go func() {
		defer wg.Done()
		assetsList, err = r.store.GetAssetList()
	}()

	var meta []banktypes.Metadata
	go func() {
		defer wg.Done()
		var metaErr error
		meta, metaErr = r.metadata.GetAllDenomsMetadata()
		if metaErr != nil {
			//just log the error
			r.logger.WithError(metaErr).Error("error getting metadata. Will continue without it")
		}
	}()
	wg.Wait()

	if err != nil {
		return nil, err
	}
	if assetsList == nil {
		return nil, fmt.Errorf("no chain registry assets found")
	}

	foundAssets := make(map[string]struct{})
	for _, a := range assetsList.Assets {
		data, err := json.Marshal(a)
		if err != nil {
			l.WithError(err).Error("error marshalling asset to cache")
			continue
		}
		foundAssets[a.Base] = struct{}{}
		err = r.cache.Set(a.Base, data, cacheTtl)
		if err != nil {
			l.WithError(err).Error("error caching asset")
			continue
		}
	}

	for _, m := range meta {
		_, ok := foundAssets[m.Base]
		if ok {
			//we already have this asset from chain registry
			continue
		}

		//the asset exists in metadata, but not in chain registry
		//try to build it and cache it
		asset := chain_registry.ChainRegistryAsset{
			Base:    m.Base,
			Name:    m.Name,
			Display: m.Display,
			Symbol:  m.Symbol,
		}

		denomUnits := make([]chain_registry.ChainRegistryAssetDenom, 0)
		for _, du := range m.DenomUnits {
			denomUnits = append(denomUnits, chain_registry.ChainRegistryAssetDenom{
				Denom:    du.Denom,
				Exponent: int(du.Exponent),
				Aliases:  du.Aliases,
			})
		}

		asset.DenomUnits = denomUnits
		data, err := json.Marshal(asset)
		if err != nil {
			l.WithError(err).Error("error marshalling metadata asset to cache")
			continue
		}

		err = r.cache.Set(m.Base, data, cacheTtl)
		if err != nil {
			l.WithError(err).Error("error caching metadata asset")
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
