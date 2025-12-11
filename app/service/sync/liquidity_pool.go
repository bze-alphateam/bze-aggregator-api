package sync

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
)

type liquidityPoolProvider interface {
	GetAllLiquidityPools() ([]tradebinTypes.LiquidityPool, error)
	GetLiquidityPool(poolId string) (*tradebinTypes.LiquidityPool, error)
}

type liquidityDataRepo interface {
	SaveOrUpdate(items []*entity.MarketLiquidityData) error
}

type LiquidityPool struct {
	marketStorage         marketRepo
	liquidityDataRepo     liquidityDataRepo
	logger                logrus.FieldLogger
	liquidityPoolProvider liquidityPoolProvider
	locker                locker
}

func NewLiquidityPoolSync(logger logrus.FieldLogger, marketStorage marketRepo, liquidityDataRepo liquidityDataRepo, provider liquidityPoolProvider, l locker) (*LiquidityPool, error) {
	if marketStorage == nil || liquidityDataRepo == nil || logger == nil || provider == nil || l == nil {
		return nil, internal.NewInvalidDependenciesErr("NewLiquidityPoolSync")
	}

	return &LiquidityPool{
		marketStorage:         marketStorage,
		liquidityDataRepo:     liquidityDataRepo,
		logger:                logger.WithField("service", "LiquidityPoolSync"),
		liquidityPoolProvider: provider,
		locker:                l,
	}, nil
}

func (lp *LiquidityPool) SyncLiquidityPools() error {
	lp.locker.Lock(getLpLockKey())
	defer lp.locker.Unlock(getLpLockKey())

	list, err := lp.liquidityPoolProvider.GetAllLiquidityPools()
	if err != nil {
		return err
	}
	if len(list) == 0 {
		lp.logger.Info("no liquidity pools found")
		return nil
	}

	lp.logger.Infof("saving %d liquidity pools", len(list))

	var marketEntities []*entity.Market
	var liquidityDataEntities []*entity.MarketLiquidityData

	existingMarkets, err := lp.marketStorage.GetMarketsMap()
	if err != nil {
		return err
	}

	for _, source := range list {
		//do not save market if it already exists
		if existingMarkets.Get(converter.GetMarketId(source.Base, source.Quote)) == nil {
			marketEntity := converter.NewMarketEntityFromLiquidityPool(&source)
			marketEntities = append(marketEntities, marketEntity)
		}

		liquidityDataEntity := converter.NewMarketLiquidityDataEntity(&source)
		liquidityDataEntities = append(liquidityDataEntities, liquidityDataEntity)
	}

	// Save or update markets
	if len(marketEntities) > 0 {
		err = lp.marketStorage.SaveIfNotExists(marketEntities)
		if err != nil {
			return err
		}
	}

	// Save or update liquidity pool data
	err = lp.liquidityDataRepo.SaveOrUpdate(liquidityDataEntities)
	if err != nil {
		return err
	}

	return nil
}

func (lp *LiquidityPool) SyncLiquidityPoolById(poolId string) error {
	lp.locker.Lock(getLpLockKey())
	defer lp.locker.Unlock(getLpLockKey())

	pool, err := lp.liquidityPoolProvider.GetLiquidityPool(poolId)
	if err != nil {
		return err
	}

	lp.logger.Infof("saving liquidity pool %s", poolId)

	marketEntity := converter.NewMarketEntityFromLiquidityPool(pool)
	liquidityDataEntity := converter.NewMarketLiquidityDataEntity(pool)

	existingMarkets, err := lp.marketStorage.GetMarketsMap()
	if err != nil {
		return err
	}

	if existingMarkets.Get(marketEntity.MarketID) == nil {
		// Save or update market
		err = lp.marketStorage.SaveIfNotExists([]*entity.Market{marketEntity})
		if err != nil {
			return err
		}
	}

	// Save or update liquidity pool data
	err = lp.liquidityDataRepo.SaveOrUpdate([]*entity.MarketLiquidityData{liquidityDataEntity})
	if err != nil {
		return err
	}

	return nil
}
