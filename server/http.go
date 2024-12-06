package server

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/bze-alphateam/bze-aggregator-api/server/factory"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
)

func Start() {
	e := echo.New()

	appCfg, err := config.NewAppConfig()
	if err != nil {
		logrus.Fatalf("could not load app config: %v", err)
	}

	logger, err := internal.NewLogger(appCfg)

	// Middleware
	e.Use(middleware.Recover())
	//generates a unique id for each request
	e.Use(middleware.RequestID())
	e.Use(middleware.CORS())

	ctrlFactory, err := factory.NewControllerFactory(logger, appCfg)
	if err != nil {
		logger.Fatalf("could not start server: %s", err)
	}

	supplyCtrl, err := ctrlFactory.GetSupplyController()
	if err != nil {
		logger.Fatalf("could not start server: %s", err)
	}

	articlesCtrl, err := ctrlFactory.GetArticlesController()
	if err != nil {
		logger.Fatalf("could not start server: %s", err)
	}

	pricesCtrl, err := ctrlFactory.GetPricesController()
	if err != nil {
		logger.Fatalf("could not start server: %s", err)
	}

	healthCtrl, err := ctrlFactory.GetHealthController()
	if err != nil {
		logger.Fatalf("could not start server: %s", err)
	}

	dexCtrl, err := ctrlFactory.GetDexController()
	if err != nil {
		logger.Fatalf("could not start server: %s", err)
	}

	// Routes
	e.GET("/api/supply/total", supplyCtrl.TotalSupplyHandler)
	e.GET("/api/supply/circulating", supplyCtrl.CirculatingSupplyHandler)
	e.GET("/api/articles/medium", articlesCtrl.MediumArticlesHandler)
	e.GET("/api/prices", pricesCtrl.PricesHandler)
	e.GET("/api/health/market", healthCtrl.DexMarketCheckHandler)
	e.GET("/api/health/aggregator", healthCtrl.DexAggregatorCheckHandler)
	e.GET("/api/health/nodes", healthCtrl.NodesCheckHandler)

	//dex related endpoints
	e.GET("/api/dex/tickers", dexCtrl.TickersHandler)
	e.GET("/api/dex/orders", dexCtrl.OrdersHandler)
	e.GET("/api/dex/history", dexCtrl.HistoryHandler)
	e.GET("/api/dex/intervals", dexCtrl.IntervalsHandler)

	// Start server
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", appCfg.Server.Port)))
}
