package controller

import (
	"net/http"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto/request"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/response"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type SwapService interface {
	GetAddressSwapHistory(address string) ([]response.HistoryTrade, error)
}

type SwapController struct {
	logger logrus.FieldLogger
	srv    SwapService
}

func NewSwapController(logger logrus.FieldLogger, service SwapService) (*SwapController, error) {
	if logger == nil || service == nil {
		return nil, internal.NewInvalidDependenciesErr("NewSwapController")
	}

	return &SwapController{
		logger: logger,
		srv:    service,
	}, nil
}

func (c *SwapController) AddressSwapHistoryHandler(ctx echo.Context) error {
	params, err := request.NewSwapHistoryParams(ctx)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, request.NewErrResponse("invalid request"))
	}

	hist, err := c.srv.GetAddressSwapHistory(params.Address)
	if err != nil {
		c.logger.WithError(err).Error("unknown error on swap history")
		return ctx.JSON(http.StatusInternalServerError, request.NewErrResponse("failed to get swap history"))
	}

	return ctx.JSON(http.StatusOK, hist)
}
