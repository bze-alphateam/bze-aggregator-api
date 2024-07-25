package server

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/bze-alphateam/bze-aggregator-api/server/factory"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
)

func Start() {
	logger := logrus.New()
	e := echo.New()

	appCfg := config.NewAppConfig()
	parsedLogLevel, err := logrus.ParseLevel(appCfg.Logging.Level)
	if err != nil {
		logger.Fatal("error on parsing logging level: %s", err)
	}

	logger.SetLevel(parsedLogLevel)

	// Middleware
	e.Use(middleware.Recover())
	//generates a unique id for each request
	e.Use(middleware.RequestID())

	ctrlFactory, err := factory.NewControllerFactory(logger)
	if err != nil {
		logger.Fatalf("could not start server: %s", err)
	}

	supplyCtrl, err := ctrlFactory.GetSupplyController()
	if err != nil {
		logger.Fatalf("could not start server: %s", err)
	}

	// Routes
	e.GET("/supply/total", supplyCtrl.TotalSupplyHandler)
	e.GET("/supply/circulating", supplyCtrl.CirculatingSupplyHandler)

	// Start server
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", appCfg.Server.Port)))
}
