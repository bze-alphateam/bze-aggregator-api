package handlers

import (
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
)

type liquidityPoolStorage interface {
	SyncLiquidityPools() error
	SyncLiquidityPoolById(poolId string) error
}

type LiquidityPoolSync struct {
	storage liquidityPoolStorage
	logger  logrus.FieldLogger
}

func NewLiquidityPoolSyncHandler(logger logrus.FieldLogger, storage liquidityPoolStorage) (*LiquidityPoolSync, error) {
	if logger == nil || storage == nil {
		return nil, internal.NewInvalidDependenciesErr("NewLiquidityPoolSyncHandler")
	}

	return &LiquidityPoolSync{logger: logger, storage: storage}, nil
}

func (s *LiquidityPoolSync) SyncLiquidityPools() {
	err := s.storage.SyncLiquidityPools()
	if err != nil {
		s.logger.WithError(err).Error("could not sync liquidity pools")
		return
	}

	s.logger.Info("liquidity pools sync finished")
}
