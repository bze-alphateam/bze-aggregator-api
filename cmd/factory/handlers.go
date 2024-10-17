package factory

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/repository"
	"github.com/bze-alphateam/bze-aggregator-api/app/service"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/client"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/data_provider"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/lock"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/sync"
	"github.com/bze-alphateam/bze-aggregator-api/cmd/handlers"
	"github.com/bze-alphateam/bze-aggregator-api/connector"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/sirupsen/logrus"
)

func GetMarketsSyncHandler(cfg *config.AppConfig, logger logrus.FieldLogger) (*handlers.MarketsSync, error) {
	locker := lock.NewInMemoryLocker()
	db, err := connector.NewDatabaseConnection()
	if err != nil {
		return nil, err
	}

	repo, err := repository.NewMarketRepository(db)
	if err != nil {
		return nil, err
	}

	storage, err := sync.NewMarketSync(logger, repo)
	if err != nil {
		return nil, err
	}

	grpc, err := client.NewGrpcClient(cfg, locker)
	if err != nil {
		return nil, err
	}

	handler, err := handlers.NewMarketsSyncHandler(logger, grpc, storage)
	if err != nil {
		return nil, err
	}

	return handler, nil
}

func GetMarketOrderSyncHandler(cfg *config.AppConfig, logger logrus.FieldLogger) (*handlers.MarketOrderSync, error) {
	locker := lock.NewInMemoryLocker()
	db, err := connector.NewDatabaseConnection()
	if err != nil {
		return nil, err
	}

	repo, err := repository.NewMarketOrderRepository(db)
	if err != nil {
		return nil, err
	}

	grpc, err := client.NewGrpcClient(cfg, locker)
	if err != nil {
		return nil, err
	}

	data, err := data_provider.NewOrderDataProvider(logger, grpc)
	if err != nil {
		return nil, err
	}

	regClient, err := client.NewChainRegistry()
	if err != nil {
		return nil, err
	}

	chainReg, err := data_provider.NewChainRegistry(logger, service.NewInMemoryCache(), regClient)
	if err != nil {
		return nil, err
	}

	orderSync, err := sync.NewOrderSync(logger, data, repo, chainReg)
	if err != nil {
		return nil, err
	}

	marketProvider, err := data_provider.NewMarketProvider(grpc, logger)
	if err != nil {
		return nil, err
	}

	handler, err := handlers.NewMarketOrderSyncHandler(logger, marketProvider, orderSync)
	if err != nil {
		return nil, err
	}

	return handler, nil
}

func GetMarketHistorySyncHandler(cfg *config.AppConfig, logger logrus.FieldLogger) (*handlers.MarketHistorySync, error) {
	locker := lock.NewInMemoryLocker()
	db, err := connector.NewDatabaseConnection()
	if err != nil {
		return nil, err
	}

	repo, err := repository.NewMarketHistoryRepository(db)
	if err != nil {
		return nil, err
	}

	grpc, err := client.NewGrpcClient(cfg, locker)
	if err != nil {
		return nil, err
	}

	data, err := data_provider.NewHistoryDataProvider(logger, grpc)
	if err != nil {
		return nil, err
	}

	regClient, err := client.NewChainRegistry()
	if err != nil {
		return nil, err
	}

	chainReg, err := data_provider.NewChainRegistry(logger, service.NewInMemoryCache(), regClient)
	if err != nil {
		return nil, err
	}

	history, err := sync.NewHistorySync(logger, data, repo, chainReg)
	if err != nil {
		return nil, err
	}

	marketProvider, err := data_provider.NewMarketProvider(grpc, logger)
	if err != nil {
		return nil, err
	}

	handler, err := handlers.NewMarketHistorySync(logger, marketProvider, history)
	if err != nil {
		return nil, err
	}

	return handler, nil
}
