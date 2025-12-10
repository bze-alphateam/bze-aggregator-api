package handlers

import (
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
)

type cleanupStorage interface {
	DeleteOldBlocks(days int) (int64, error)
}

type Cleanup struct {
	storage cleanupStorage
	logger  logrus.FieldLogger
}

func NewCleanupHandler(logger logrus.FieldLogger, storage cleanupStorage) (*Cleanup, error) {
	if logger == nil || storage == nil {
		return nil, internal.NewInvalidDependenciesErr("NewCleanupHandler")
	}

	return &Cleanup{logger: logger, storage: storage}, nil
}

func (c *Cleanup) CleanupOldBlocks(days int) error {
	c.logger.Infof("cleaning up blocks older than %d days", days)

	deletedCount, err := c.storage.DeleteOldBlocks(days)
	if err != nil {
		c.logger.WithError(err).Error("failed to cleanup old blocks")
		return err
	}

	if deletedCount == 0 {
		c.logger.Info("no blocks to cleanup")
	} else {
		c.logger.Infof("successfully deleted %d blocks and related data", deletedCount)
	}

	return nil
}
