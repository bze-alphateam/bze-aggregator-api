package controller

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/request"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"net/http"
)

type MarketHealthCheckService interface {
	GetMarketHealth(marketId string, minutesAgo int) dto.MarketHealth
}

type HealthCheckController struct {
	logger  logrus.FieldLogger
	service MarketHealthCheckService
}

func NewHealthCheckController(logger logrus.FieldLogger, service MarketHealthCheckService) (*HealthCheckController, error) {
	if logger == nil || service == nil {
		return nil, internal.NewInvalidDependenciesErr("NewHealthCheckController")
	}

	return &HealthCheckController{
		logger:  logger,
		service: service,
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

func (c *HealthCheckController) getMethodLogger(method string) logrus.FieldLogger {
	return c.logger.WithField("struct", "HealthCheckController").WithField("method", method)
}
