package handlers

import (
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
)

type swapEventStorage interface {
	SyncSwapEvents(batchSize int) (pools []string, err error)
}

type SyncEvents struct {
	storage swapEventStorage
	logger  logrus.FieldLogger
}

func NewSyncEventsHandler(logger logrus.FieldLogger, storage swapEventStorage) (*SyncEvents, error) {
	if logger == nil || storage == nil {
		return nil, internal.NewInvalidDependenciesErr("NewSyncEventsHandler")
	}

	return &SyncEvents{logger: logger, storage: storage}, nil
}

func (s *SyncEvents) SyncSwapEvents(batchSize int) error {
	s.logger.Infof("syncing swap events with batch size %d", batchSize)

	pools, err := s.storage.SyncSwapEvents(batchSize)
	if err != nil {
		s.logger.WithError(err).Error("failed to sync swap events")
		return err
	}

	processedCount := len(pools)
	if processedCount == 0 {
		s.logger.Info("no swap events to process")
	} else {
		s.logger.Infof("successfully synced swap events for %d pools", processedCount)
	}

	return nil
}
