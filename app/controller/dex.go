package controller

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/request"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"net/http"
)

type TickersService interface {
	GetTickers() ([]*dto.Ticker, error)
	GetCoingeckoTickers() ([]*dto.CoingeckoTicker, error)
}

type Dex struct {
	logger  logrus.FieldLogger
	tickers TickersService
}

func NewDexController(logger logrus.FieldLogger, service TickersService) (*Dex, error) {
	if logger == nil || service == nil {
		return nil, internal.NewInvalidDependenciesErr("NewDexController")
	}

	return &Dex{
		logger:  logger,
		tickers: service,
	}, nil
}

func (d *Dex) TickersHandler(ctx echo.Context) error {
	l := d.getMethodLogger("TickersHandler")

	params, err := request.NewTickersParams(ctx)
	if err != nil {
		l.WithError(err).Error("error when creating request parameters")

		return ctx.JSON(http.StatusBadRequest, request.NewErrResponse("invalid request"))
	}

	if params.IsCoingeckoFormat() {
		data, err := d.tickers.GetCoingeckoTickers()
		if err != nil {
			l.WithError(err).Error("error when getting tickers")

			return ctx.JSON(http.StatusInternalServerError, request.NewUnknownErrorResponse())
		}

		return ctx.JSON(http.StatusOK, data)
	}

	data, err := d.tickers.GetTickers()
	if err != nil {
		l.WithError(err).Error("error when getting tickers")

		return ctx.JSON(http.StatusInternalServerError, request.NewUnknownErrorResponse())
	}

	return ctx.JSON(http.StatusOK, data)
}

func (d *Dex) getMethodLogger(method string) logrus.FieldLogger {
	return d.logger.WithField("struct", "DexController").WithField("method", method)
}
