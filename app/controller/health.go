package controller

import (
	"fmt"
	"net/http"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/request"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/response"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type BalanceHealthCheckService interface {
	CheckBalances(params *request.BalanceHealthParams) []dto.AddressHealthCheck
}

type MarketHealthCheckService interface {
	GetMarketHealth(marketId string, minutesAgo int) dto.MarketHealth
	GetAggregatorHealth(minutesAgo int) dto.AggregatorHealth
	GetNodesHealth() dto.NodesHealth
}

type HealthCheckController struct {
	logger         logrus.FieldLogger
	service        MarketHealthCheckService
	balanceChecker BalanceHealthCheckService
}

func NewHealthCheckController(logger logrus.FieldLogger, service MarketHealthCheckService, balance BalanceHealthCheckService) (*HealthCheckController, error) {
	if logger == nil || service == nil || balance == nil {
		return nil, internal.NewInvalidDependenciesErr("NewHealthCheckController")
	}

	return &HealthCheckController{
		logger:         logger,
		service:        service,
		balanceChecker: balance,
	}, nil
}

func (c *HealthCheckController) DexMarketCheckHandler(ctx echo.Context) error {
	l := c.getMethodLogger("DexMarketCheckHandler")

	params, err := request.NewMarketHealthRequest(ctx)
	if err != nil {
		l.WithError(err).Error("error when creating request parameters")

		return ctx.JSON(http.StatusBadRequest, request.NewErrResponse("invalid request"))
	}

	if err = params.Validate(); err != nil {
		l.WithError(err).Info("validation failed")

		return ctx.JSON(http.StatusBadRequest, request.NewErrResponse("invalid parameters"))
	}

	return ctx.JSON(http.StatusOK, c.service.GetMarketHealth(params.MarketId, params.Minutes))
}

func (c *HealthCheckController) DexAggregatorCheckHandler(ctx echo.Context) error {
	l := c.getMethodLogger("DexAggregatorCheckHandler")

	params, err := request.NewAggregatorHealthRequest(ctx)
	if err != nil {
		l.WithError(err).Error("error when creating request parameters")

		return ctx.JSON(http.StatusBadRequest, request.NewErrResponse("invalid request"))
	}

	return ctx.JSON(http.StatusOK, c.service.GetAggregatorHealth(params.Minutes))
}

func (c *HealthCheckController) NodesCheckHandler(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, c.service.GetNodesHealth())
}

func (c *HealthCheckController) getMethodLogger(method string) logrus.FieldLogger {
	return c.logger.WithField("struct", "HealthCheckController").WithField("method", method)
}

func (c *HealthCheckController) CheckBalancesHandler(ctx echo.Context) error {
	params, err := request.NewBalanceHealthParams(ctx)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, request.NewErrResponse("invalid request"))
	}

	result := response.BalanceHealthResponse{
		IsHealthy: true,
		Errors:    "",
	}
	checkResult := c.balanceChecker.CheckBalances(params)
	for _, cr := range checkResult {
		if cr.IsHealthy {
			continue
		}
		result.IsHealthy = false

		addrErr := fmt.Sprintf("[%s]: [%s]", cr.Address, cr.Error)
		//add first error
		if result.Errors == "" {
			result.Errors = addrErr
			continue
		}

		result.Errors = fmt.Sprintf("%s; %s", result.Errors, addrErr)
	}

	return ctx.JSON(http.StatusOK, result)
}
