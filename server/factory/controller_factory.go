package factory

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/controller"
	"github.com/bze-alphateam/bze-aggregator-api/app/repository"
	appService "github.com/bze-alphateam/bze-aggregator-api/app/service"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/client"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/data_provider"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/dex"
	"github.com/bze-alphateam/bze-aggregator-api/connector"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/sirupsen/logrus"
)

type ControllerFactory struct {
	logger logrus.FieldLogger
	config *config.AppConfig
}

func NewControllerFactory(logger logrus.FieldLogger, cfg *config.AppConfig) (*ControllerFactory, error) {
	if logger == nil || cfg == nil {
		return nil, fmt.Errorf("could not instantiate controller factory: invalid dependencies")
	}

	return &ControllerFactory{
		logger: logger,
		config: cfg,
	}, nil
}

func (c *ControllerFactory) GetSupplyController() (*controller.SupplyController, error) {
	cache := appService.NewInMemoryCache()
	if cache == nil {
		return nil, fmt.Errorf("could not instantiate in memory cache")
	}

	dp, err := client.NewBlockchainQueryClient(c.config.Blockchain.RestHost)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate blockchain query client: %w", err)
	}

	regClient, err := client.NewChainRegistry()
	if err != nil {
		return nil, err
	}

	chainReg, err := data_provider.NewChainRegistry(c.logger, cache, regClient)
	if err != nil {
		return nil, err
	}

	service, err := appService.NewSupplyService(c.logger, cache, dp, chainReg)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate supply service: %w", err)
	}

	return controller.NewSupplyController(c.logger, service)
}

func (c *ControllerFactory) GetArticlesController() (*controller.ArticlesController, error) {
	cache := appService.NewInMemoryCache()
	if cache == nil {
		return nil, fmt.Errorf("could not instantiate in memory cache")
	}

	service, err := appService.NewMediumService(c.logger, cache)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate supply service: %w", err)
	}

	return controller.NewArticlesController(c.logger, service)
}

func (c *ControllerFactory) GetPricesController() (*controller.PricesController, error) {
	cache := appService.NewInMemoryCache()
	if cache == nil {
		return nil, fmt.Errorf("could not instantiate in memory cache")
	}

	cgClient, err := client.NewCoingeckoClient(c.config.Coingecko.Host, c.config.Prices.Denominations)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate coingecko client: %w", err)
	}

	service, err := appService.NewPricesService(cache, cgClient, c.logger)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate prices service: %w", err)
	}

	return controller.NewPricesController(c.logger, service)
}

func (c *ControllerFactory) GetHealthController() (*controller.HealthCheckController, error) {
	cache := appService.NewInMemoryCache()
	if cache == nil {
		return nil, fmt.Errorf("could not instantiate in memory cache")
	}

	dp, err := client.NewBlockchainQueryClient(c.config.Blockchain.RestHost)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate blockchain query client: %w", err)
	}

	db, err := connector.NewDatabaseConnection()
	if err != nil {
		return nil, err
	}

	repo, err := repository.NewMarketHistoryRepository(db)
	if err != nil {
		return nil, err
	}

	service, err := appService.NewHealthService(c.logger, cache, dp, repo)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate prices service: %w", err)
	}

	return controller.NewHealthCheckController(c.logger, service)
}

func (c *ControllerFactory) GetDexController() (*controller.Dex, error) {
	db, err := connector.NewDatabaseConnection()
	if err != nil {
		return nil, err
	}

	mRepo, err := repository.NewMarketRepository(db)
	if err != nil {
		return nil, err
	}

	iRepo, err := repository.NewMarketIntervalRepository(db)
	if err != nil {
		return nil, err
	}

	oRepo, err := repository.NewMarketOrderRepository(db)
	if err != nil {
		return nil, err
	}

	hRepo, err := repository.NewMarketHistoryRepository(db)
	if err != nil {
		return nil, err
	}

	tickers, err := dex.NewTickersService(c.logger, mRepo, iRepo, oRepo)
	if err != nil {
		return nil, err
	}

	orders, err := dex.NewOrdersService(c.logger, oRepo, mRepo)
	if err != nil {
		return nil, err
	}

	history, err := dex.NewHistoryService(c.logger, hRepo)
	if err != nil {
		return nil, err
	}

	intervals, err := dex.NewIntervals(iRepo, c.logger, mRepo)
	if err != nil {
		return nil, err
	}

	return controller.NewDexController(c.logger, tickers, orders, history, intervals)
}
