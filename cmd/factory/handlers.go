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
	locker := lock.GetInMemoryLocker()
	db, err := connector.NewDatabaseConnection()
	if err != nil {
		return nil, err
	}

	repo, err := repository.NewMarketRepository(db)
	if err != nil {
		return nil, err
	}

	grpc, err := client.NewGrpcClient(cfg, locker)
	if err != nil {
		return nil, err
	}

	provider, err := data_provider.NewMarketProvider(grpc, logger)
	if err != nil {
		return nil, err
	}

	storage, err := sync.NewMarketSync(logger, repo, provider)
	if err != nil {
		return nil, err
	}

	handler, err := handlers.NewMarketsSyncHandler(logger, storage)
	if err != nil {
		return nil, err
	}

	return handler, nil
}

func GetMarketOrderSyncHandler(cfg *config.AppConfig, logger logrus.FieldLogger) (*handlers.MarketOrderSync, error) {
	locker := lock.GetInMemoryLocker()
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

	orderSync, err := sync.NewOrderSync(logger, data, repo, chainReg, lock.GetInMemoryLocker())
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
	locker := lock.GetInMemoryLocker()
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

	history, err := sync.NewHistorySync(logger, data, repo, chainReg, lock.GetInMemoryLocker())
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

func GetMarketIntervalSyncHandler(cfg *config.AppConfig, logger logrus.FieldLogger) (*handlers.MarketIntervalSync, error) {
	locker := lock.GetInMemoryLocker()
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

	iRepo, err := repository.NewMarketIntervalRepository(db)
	if err != nil {
		return nil, err
	}

	history, err := sync.NewIntervalSync(logger, repo, locker, iRepo)
	if err != nil {
		return nil, err
	}

	marketProvider, err := data_provider.NewMarketProvider(grpc, logger)
	if err != nil {
		return nil, err
	}

	handler, err := handlers.NewMarketIntervalSync(logger, marketProvider, history)
	if err != nil {
		return nil, err
	}

	return handler, nil
}

func GetSyncListener(cfg *config.AppConfig, logger logrus.FieldLogger) (*handlers.Listener, error) {
	locker := lock.GetInMemoryLocker()
	db, err := connector.NewDatabaseConnection()
	if err != nil {
		return nil, err
	}

	mRepo, err := repository.NewMarketRepository(db)
	if err != nil {
		return nil, err
	}

	grpc, err := client.NewGrpcClient(cfg, locker)
	if err != nil {
		return nil, err
	}

	mProvider, err := data_provider.NewMarketProvider(grpc, logger)
	if err != nil {
		return nil, err
	}

	market, err := sync.NewMarketSync(logger, mRepo, mProvider)
	if err != nil {
		return nil, err
	}

	hRepo, err := repository.NewMarketHistoryRepository(db)
	if err != nil {
		return nil, err
	}

	iRepo, err := repository.NewMarketIntervalRepository(db)
	if err != nil {
		return nil, err
	}

	interval, err := sync.NewIntervalSync(logger, hRepo, locker, iRepo)
	if err != nil {
		return nil, err
	}

	hData, err := data_provider.NewHistoryDataProvider(logger, grpc)
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

	history, err := sync.NewHistorySync(logger, hData, hRepo, chainReg, locker)
	if err != nil {
		return nil, err
	}

	oData, err := data_provider.NewOrderDataProvider(logger, grpc)
	if err != nil {
		return nil, err
	}

	oRepo, err := repository.NewMarketOrderRepository(db)
	if err != nil {
		return nil, err
	}

	order, err := sync.NewOrderSync(logger, oData, oRepo, chainReg, locker)
	if err != nil {
		return nil, err
	}

	return handlers.NewListener(logger, history, interval, order, market, mProvider, locker)
}
