package handlers

import (
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
)

type historyStorage interface {
	SyncHistory(market *types.Market, batchSize uint64) error
}

type MarketHistorySync struct {
	mProvider marketProvider
	storage   historyStorage
	logger    logrus.FieldLogger
}

func NewMarketHistorySync(logger logrus.FieldLogger, provider marketProvider, storage historyStorage) (*MarketHistorySync, error) {
	if logger == nil || provider == nil || storage == nil {
		return nil, internal.NewInvalidDependenciesErr("NewMarketHistorySync")
	}

	return &MarketHistorySync{
		mProvider: provider,
		storage:   storage,
		logger:    logger,
	}, nil
}

func (m *MarketHistorySync) SyncHistory(marketId string) error {
	return syncMarket(marketId, m.mProvider, m.logger, m.syncMarket)
}

func (m *MarketHistorySync) SyncAll() {
	syncAll(m.mProvider, m.logger, m.syncMarket)
}

func (m *MarketHistorySync) syncMarket(market *types.Market) error {
	return m.storage.SyncHistory(market, 0)
}
