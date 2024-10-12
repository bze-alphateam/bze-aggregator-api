package handlers

import (
	"fmt"
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
	res := s.getMarkets()
	if len(res) == 0 {
		s.logger.Error("could not fetch markets")
		return
	}

	for _, m := range res {
		mId := converter.GetMarketId(m.GetBase(), m.GetQuote())
		l := s.logger.WithField("market_id", mId)

		err := s.syncMarket(&m)
		if err != nil {
			l.WithError(err).Error("could not sync market orders")
			continue
		}

		l.Info("market orders synced")
	}
}

func (s *MarketOrderSync) getMarkets() []types.Market {
	res, err := s.mProvider.GetAllMarkets()
	if err != nil {
		s.logger.WithError(err).Error("could not get markets")
		return nil
	}
	s.logger.Info("markets fetched")

	return res
}

func (s *MarketOrderSync) SyncMarketOrders(marketId string) error {
	all := s.getMarkets()
	if len(all) == 0 {
		return fmt.Errorf("no markets found")
	}

	for _, m := range all {
		mId := converter.GetMarketId(m.GetBase(), m.GetQuote())
		if mId == marketId {
			return s.syncMarket(&m)
		}
	}

	return fmt.Errorf("market %s not found", marketId)
}

func (s *MarketOrderSync) syncMarket(market *types.Market) error {
	l := s.logger.WithField("market_id", converter.GetMarketId(market.GetBase(), market.GetQuote()))
	l.Info("preparing to sync market")

	return s.storage.SyncMarket(market)
}
