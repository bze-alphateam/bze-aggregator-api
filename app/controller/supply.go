package controller

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/request"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"net/http"
)

type SupplyService interface {
	GetTotalSupply(denom string) (string, error)
	GetCirculatingSupply(denom string) (string, error)
}

type SupplyController struct {
	service SupplyService
	logger  logrus.FieldLogger
}

func NewSupplyController(logger logrus.FieldLogger, service SupplyService) (*SupplyController, error) {
	if logger == nil || service == nil {
		return nil, internal.NewInvalidDependenciesErr("NewSupplyController")
	}

	return &SupplyController{service: service, logger: logger.WithField("struct", "SupplyController")}, nil
}

func (c *SupplyController) TotalSupplyHandler(ctx echo.Context) error {
	l := c.logger.WithField("func", "TotalSupplyHandler")
	params, err := request.NewSupplyParams(ctx)
	if err != nil {
		l.WithError(err).Error("failed to create total supply params")

		return ctx.String(http.StatusBadRequest, "invalid request")
	}

	supply, err := c.service.GetTotalSupply(params.Denom)
	if err != nil {
		return ctx.String(http.StatusBadRequest, err.Error())
	}

	return ctx.String(http.StatusOK, supply)
}

func (c *SupplyController) CirculatingSupplyHandler(ctx echo.Context) error {
	l := c.logger.WithField("func", "CirculatingSupplyHandler")
	params, err := request.NewSupplyParams(ctx)
	if err != nil {
		l.WithError(err).Error("failed to create circulating supply params")

		return ctx.String(http.StatusBadRequest, "invalid request")
	}

	supply, err := c.service.GetCirculatingSupply(params.Denom)
	if err != nil {
		return ctx.String(http.StatusBadRequest, err.Error())
	}

	return ctx.String(http.StatusOK, supply)
}
