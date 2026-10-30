package server

import (
	"fmt"
	"net/http"

	"github.com/bze-alphateam/bze-aggregator-api/app/controller"
	appService "github.com/bze-alphateam/bze-aggregator-api/app/service"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/client"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/health"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/bze-alphateam/bze-aggregator-api/server/scanproxy"
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

	scanProxy, err := scanproxy.New(scanproxy.Config{Host: appCfg.Scan.Host}, &http.Client{}, logger)
	if err != nil {
		logger.Fatalf("could not start server: %s", err)
	}

	healthCtrl, err := newHealthController(logger, appCfg)
	if err != nil {
		logger.Fatalf("could not start server: %s", err)
	}

	// Routes: the health checks are answered here, everything else under /api
	// comes from BZE Scan (echo matches the static routes before the wildcard).
	e.GET("/api/health/market", healthCtrl.DexMarketCheckHandler)
	e.GET("/api/health/nodes", healthCtrl.NodesCheckHandler)
	e.POST("/api/health/balances", healthCtrl.CheckBalancesHandler)
	e.GET("/api/*", scanProxy.Handler())

	// Start server
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", appCfg.Server.Port)))
}

func newHealthController(logger logrus.FieldLogger, cfg *config.AppConfig) (*controller.HealthCheckController, error) {
	dp, err := client.NewBlockchainQueryClient(cfg.Blockchain.RestHost)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate blockchain query client: %w", err)
	}

	var healthClients map[string]appService.NodeInfoClient
	if len(cfg.Blockchain.HealthNodes) > 0 {
		healthClients = make(map[string]appService.NodeInfoClient, len(cfg.Blockchain.HealthNodes))
		for hostName, hostAddr := range cfg.Blockchain.HealthNodes {
			rpc, err := client.GetRpcClient(hostAddr)
			if err != nil {
				return nil, err
			}

			healthClients[hostName] = rpc
		}
	}

	service, err := appService.NewHealthService(logger, appService.NewInMemoryCache(), dp, healthClients)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate health service: %w", err)
	}

	balanceHealthService, err := health.NewBalancesHealth(cfg.PrefixedEndpoints)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate balance health service: %w", err)
	}

	return controller.NewHealthCheckController(logger, service, balanceHealthService)
}
