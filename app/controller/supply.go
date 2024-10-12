package controller

import (
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"net/http"
)

type SupplyService interface {
	GetTotalSupply() string
	GetCirculatingSupply() string
}

type SupplyController struct {
	service SupplyService
	logger  logrus.FieldLogger
}

func NewSupplyController(logger logrus.FieldLogger, service SupplyService) (*SupplyController, error) {
	if logger == nil || service == nil {
		return nil, internal.NewInvalidDependenciesErr("NewSupplyController")
	}

	return &SupplyController{service: service, logger: logger}, nil
}

func (c *SupplyController) TotalSupplyHandler(ctx echo.Context) error {

	return ctx.String(http.StatusOK, c.service.GetTotalSupply())
}

func (c *SupplyController) CirculatingSupplyHandler(ctx echo.Context) error {

	return ctx.String(http.StatusOK, c.service.GetCirculatingSupply())
}
