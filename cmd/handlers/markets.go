package handlers

import (
	"context"
	"fmt"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/sirupsen/logrus"
)

type grpc interface {
	GetTradebinQueryClient() (tradebinTypes.QueryClient, error)
	CloseConnection()
}

type marketStorage interface {
	SaveMarkets(market []tradebinTypes.Market) error
}

type MarketsSync struct {
	grpc    grpc
	storage marketStorage
	logger  logrus.FieldLogger
}

func NewMarketsSync(logger logrus.FieldLogger, grpc grpc, storage marketStorage) (*MarketsSync, error) {
	if grpc == nil || logger == nil || storage == nil {
		return nil, fmt.Errorf("invalid dependencies provided to NewMarketsSync")
	}

	return &MarketsSync{grpc: grpc, logger: logger, storage: storage}, nil
}

func (s *MarketsSync) SyncMarkets() {
	defer s.grpc.CloseConnection()
	s.logger.Info("getting tradebin query client")
	qc, err := s.grpc.GetTradebinQueryClient()
	if err != nil {
		s.logger.WithError(err).Error("could not get tradebin query client")
		return
	}

	params := s.getMarketsParams()
	s.logger.Info("fetching markets from blockchain")

	res, err := qc.MarketAll(context.Background(), params)
	if err != nil {
		s.logger.WithError(err).Error("could not get markets")
		return
	}
	s.logger.Info("markets fetched")

	err = s.storage.SaveMarkets(res.GetMarket())
	if err != nil {
		s.logger.WithError(err).Error("could not save markets")
		return
	}

	s.logger.Info("markets sync finished")
}

func (s *MarketsSync) getMarketsParams() *tradebinTypes.QueryAllMarketRequest {
	return &tradebinTypes.QueryAllMarketRequest{
		Pagination: &query.PageRequest{
			Limit: 10000,
		},
	}
}
