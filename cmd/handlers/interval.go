package handlers

import (
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
)

type intervalStorage interface {
	SyncIntervals(market *types.Market) error
}

type MarketIntervalSync struct {
	mProvider marketProvider
	storage   intervalStorage
	logger    logrus.FieldLogger
}

func NewMarketIntervalSync(logger logrus.FieldLogger, provider marketProvider, storage intervalStorage) (*MarketIntervalSync, error) {
	if logger == nil || provider == nil || storage == nil {
		return nil, internal.NewInvalidDependenciesErr("NewMarketIntervalSync")
	}

	return &MarketIntervalSync{
		mProvider: provider,
		storage:   storage,
		logger:    logger,
	}, nil
}

func (m *MarketIntervalSync) SyncIntervals(marketId string) error {
	return syncMarket(marketId, m.mProvider, m.logger, m.syncInterval)
}

func (m *MarketIntervalSync) SyncAll() {
	syncAll(m.mProvider, m.logger, m.syncInterval)
}

func (m *MarketIntervalSync) syncInterval(market *types.Market) error {
	return m.storage.SyncIntervals(market)
}
