package factory

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/controller"
	appService "github.com/bze-alphateam/bze-aggregator-api/app/service"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/client"
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

	service, err := appService.NewSupplyService(c.logger, cache, dp)
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
