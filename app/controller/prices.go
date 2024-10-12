package controller

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"net/http"
)

type PricesService interface {
	GetPrices() []dto.CoinPrice
}

type PricesController struct {
	service PricesService
	logger  logrus.FieldLogger
}

func NewPricesController(logger logrus.FieldLogger, service PricesService) (*PricesController, error) {
	if logger == nil || service == nil {
		return nil, internal.NewInvalidDependenciesErr("NewPricesController")
	}

	return &PricesController{service: service, logger: logger}, nil
}

func (c *PricesController) PricesHandler(ctx echo.Context) error {

	return ctx.JSON(http.StatusOK, c.service.GetPrices())
}
