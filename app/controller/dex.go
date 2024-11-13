package controller

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/request"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/response"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"net/http"
)

type historyService interface {
	GetHistory(params *request.HistoryParams) ([]response.HistoryTrade, error)
	GetCoingeckoHistory(params *request.HistoryParams) (*response.CoingeckoHistory, error)
}

type ordersService interface {
	GetMarketOrders(marketId string, depth int) (*response.Orders, error)
	GetCoingeckoMarketOrders(marketId string, depth int) (*response.CoingeckoOrders, error)
}

type tickersService interface {
	GetTickers() ([]*response.Ticker, error)
	GetCoingeckoTickers() ([]*response.CoingeckoTicker, error)
}

type Dex struct {
	logger  logrus.FieldLogger
	tickers tickersService
	orders  ordersService
	history historyService
}

func NewDexController(logger logrus.FieldLogger, service tickersService, orders ordersService, history historyService) (*Dex, error) {
	if logger == nil || service == nil || orders == nil || history == nil {
		return nil, internal.NewInvalidDependenciesErr("NewDexController")
	}

	return &Dex{
		logger:  logger,
		tickers: service,
		orders:  orders,
		history: history,
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

func (d *Dex) OrdersHandler(ctx echo.Context) error {
	l := d.getMethodLogger("OrdersHandler")

	params, err := request.NewOrdersParams(ctx)
	if err != nil {
		l.WithError(err).Error("error when creating request parameters")

		return ctx.JSON(http.StatusBadRequest, request.NewErrResponse("invalid request"))
	}

	if err = params.Validate(); err != nil {
		l.WithError(err).Info("error when creating request parameters")

		return ctx.JSON(http.StatusBadRequest, request.NewErrResponse(err.Error()))
	}

	marketId := params.MustGetMarketId()
	if params.IsCoingeckoFormat() {
		data, err := d.orders.GetCoingeckoMarketOrders(marketId, params.Depth)
		if err != nil {
			l.WithError(err).Error("error when getting orders")

			return ctx.JSON(http.StatusBadRequest, request.NewUnknownErrorResponse())
		}

		return ctx.JSON(http.StatusOK, data)
	}

	data, err := d.orders.GetMarketOrders(marketId, params.Depth)
	if err != nil {
		l.WithError(err).Error("error when getting orders")

		return ctx.JSON(http.StatusBadRequest, request.NewUnknownErrorResponse())
	}

	return ctx.JSON(http.StatusOK, data)
}

func (d *Dex) HistoryHandler(ctx echo.Context) error {
	l := d.getMethodLogger("HistoryHandler")

	params, err := request.NewHistoryParams(ctx)
	if err != nil {
		l.WithError(err).Error("error when creating request parameters")

		return ctx.JSON(http.StatusBadRequest, request.NewErrResponse("invalid request"))
	}

	if err = params.Validate(); err != nil {
		l.WithError(err).Info("error when creating request parameters")

		return ctx.JSON(http.StatusBadRequest, request.NewErrResponse(err.Error()))
	}

	if params.IsCoingeckoFormat() {
		data, err := d.history.GetCoingeckoHistory(params)
		if err != nil {
			l.WithError(err).Error("error when getting history")

			return ctx.JSON(http.StatusBadRequest, request.NewUnknownErrorResponse())
		}

		return ctx.JSON(http.StatusOK, data)
	}

	data, err := d.history.GetHistory(params)
	if err != nil {
		l.WithError(err).Error("error when getting history")

		return ctx.JSON(http.StatusBadRequest, request.NewUnknownErrorResponse())
	}

	return ctx.JSON(http.StatusOK, data)
}

func (d *Dex) getMethodLogger(method string) logrus.FieldLogger {
	return d.logger.WithField("struct", "DexController").WithField("method", method)
}
