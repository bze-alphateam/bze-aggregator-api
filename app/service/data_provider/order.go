package data_provider

import (
	"context"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/sirupsen/logrus"
)

const (
	buy  = "buy"
	sell = "sell"
)

type Order struct {
	provider clientProvider
	logger   logrus.FieldLogger
}

func NewOrderDataProvider(logger logrus.FieldLogger, provider clientProvider) (*Order, error) {
	if provider == nil || logger == nil {
		return nil, internal.NewInvalidDependenciesErr("NewOrderDataProvider")
	}

	return &Order{provider: provider, logger: logger}, nil
}

func (o *Order) GetActiveBuyOrders(marketId string) ([]types.AggregatedOrder, error) {
	return o.getAggregatedOrders(marketId, buy)
}

func (o *Order) GetActiveSellOrders(marketId string) ([]types.AggregatedOrder, error) {
	return o.getAggregatedOrders(marketId, sell)
}

func (o *Order) getAggregatedOrders(marketId, orderType string) ([]types.AggregatedOrder, error) {
	o.logger.Info("getting tradebin query client")
	qc, err := o.provider.GetTradebinQueryClient()
	if err != nil {
		return nil, err
	}

	params := o.getAggregatedOrdersQueryParams(marketId, orderType)
	o.logger.Info("fetching aggregated orders from blockchain")

	res, err := qc.MarketAggregatedOrders(context.Background(), params)
	if err != nil {
		return nil, err
	}
	o.logger.Info("aggregated orders fetched")

	return res.GetList(), nil
}

func (o *Order) getAggregatedOrdersQueryParams(marketId, orderType string) *types.QueryMarketAggregatedOrdersRequest {
	var reverse bool
	if orderType == "buy" {
		reverse = true
	}

	return &types.QueryMarketAggregatedOrdersRequest{
		Market:    marketId,
		OrderType: orderType,
		Pagination: &query.PageRequest{
			Limit:   10000,
			Reverse: reverse,
		},
	}
}
