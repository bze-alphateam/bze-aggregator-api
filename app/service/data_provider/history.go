package data_provider

import (
	"context"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/sirupsen/logrus"
)

type History struct {
	provider clientProvider
	logger   logrus.FieldLogger
}

func NewHistoryDataProvider(logger logrus.FieldLogger, provider clientProvider) (*History, error) {
	if provider == nil || logger == nil {
		return nil, internal.NewInvalidDependenciesErr("NewHistoryDataProvider")
	}

	return &History{provider: provider, logger: logger}, nil
}

func (o *History) GetMarketHistory(marketId string, limit uint64, key string) ([]types.HistoryOrder, error) {
	qc, err := o.provider.GetTradebinQueryClient()
	if err != nil {
		return nil, err
	}

	params := o.getHistoryQueryParams(marketId, limit, key)
	o.logger.Info("fetching history orders from blockchain")

	res, err := qc.MarketHistory(context.Background(), params)
	if err != nil {
		return nil, err
	}
	o.logger.Info("aggregated orders fetched")

	return res.GetList(), nil
}
func (o *History) getHistoryQueryParams(marketId string, limit uint64, key string) *types.QueryMarketHistoryRequest {
	res := types.QueryMarketHistoryRequest{
		Market: marketId,
		Pagination: &query.PageRequest{
			Limit:   limit,
			Reverse: true,
		},
	}

	if key != "" {
		res.Pagination.Key = []byte(key)
	}

	return &res
}
