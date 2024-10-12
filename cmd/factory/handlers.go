package factory

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/repository"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/client"
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

	handler, err := handlers.NewMarketsSync(logger, grpc, storage)
	if err != nil {
		return nil, err
	}

	return handler, nil
}
