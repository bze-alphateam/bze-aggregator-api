package handlers

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
)

type marketProvider interface {
	GetAllMarkets() ([]types.Market, error)
}

type orderStorage interface {
	SyncMarket(market *types.Market) error
}

type MarketOrderSync struct {
	mProvider marketProvider
	storage   orderStorage
	logger    logrus.FieldLogger
}

func NewMarketOrderSyncHandler(logger logrus.FieldLogger, mProvider marketProvider, storage orderStorage) (*MarketOrderSync, error) {
	if mProvider == nil || logger == nil || storage == nil {
		return nil, internal.NewInvalidDependenciesErr("NewMarketOrderSyncHandler")
	}

	return &MarketOrderSync{mProvider: mProvider, logger: logger, storage: storage}, nil
}

func (s *MarketOrderSync) SyncAll() {
	syncAll(s.mProvider, s.logger, s.syncMarket)
}

func (s *MarketOrderSync) SyncMarketOrders(marketId string) error {
	return syncMarket(marketId, s.mProvider, s.logger, s.syncMarket)
}

func (s *MarketOrderSync) syncMarket(market *types.Market) error {
	l := s.logger.WithField("market_id", converter.GetMarketId(market.GetBase(), market.GetQuote()))
	l.Info("preparing to sync market")

	return s.storage.SyncMarket(market)
}
