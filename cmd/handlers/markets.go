package handlers

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/data_provider"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
)

type grpc interface {
	GetTradebinQueryClient() (tradebinTypes.QueryClient, error)
	CloseConnection()
}

type marketDataProvider interface {
	GetAllMarkets() ([]tradebinTypes.Market, error)
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
	//initializing market provider here so we can control grpc connection closing
	dp, err := data_provider.NewMarketProvider(s.grpc, s.logger)
	if err != nil {
		s.logger.WithError(err).Error("error initializing data provider")
		return
	}

	res, err := dp.GetAllMarkets()
	if err != nil {
		s.logger.WithError(err).Error("could not get markets")
		return
	}
	s.logger.Info("markets fetched")

	err = s.storage.SaveMarkets(res)
	if err != nil {
		s.logger.WithError(err).Error("could not save markets")
		return
	}

	s.logger.Info("markets sync finished")
}
