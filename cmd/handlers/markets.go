package handlers

import (
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
)

type marketStorage interface {
	SyncMarkets() error
}

type MarketsSync struct {
	storage marketStorage
	logger  logrus.FieldLogger
}

func NewMarketsSyncHandler(logger logrus.FieldLogger, storage marketStorage) (*MarketsSync, error) {
	if logger == nil || storage == nil {
		return nil, internal.NewInvalidDependenciesErr("NewMarketsSyncHandler")
	}

	return &MarketsSync{logger: logger, storage: storage}, nil
}

func (s *MarketsSync) SyncMarkets() {
	err := s.storage.SyncMarkets()
	if err != nil {
		s.logger.WithError(err).Error("could not save markets")
		return
	}

	s.logger.Info("markets sync finished")
}
