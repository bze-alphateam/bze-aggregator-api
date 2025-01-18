package data_provider

import (
	"context"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/sirupsen/logrus"
	"time"
)

type History struct {
	provider clientProvider
	logger   logrus.FieldLogger
}

func NewHistoryDataProvider(logger logrus.FieldLogger, provider clientProvider) (*History, error) {
	if provider == nil || logger == nil {
		return nil, internal.NewInvalidDependenciesErr("NewHistoryDataProvider")
	}

	return &History{
		provider: provider,
		logger:   logger.WithField("service", "DataProvider.History"),
	}, nil
}

func (o *History) GetMarketHistory(marketId string, limit uint64, key string) ([]types.HistoryOrder, string, error) {
	qc, err := o.provider.GetTradebinQueryClient()
	if err != nil {
		return nil, "", err
	}

	params := o.getHistoryQueryParams(marketId, limit, key, true)
	o.logger.Info("fetching history orders from blockchain")
	o.logger.WithField("params", params).Info("using params to get market history")

	res, err := qc.MarketHistory(context.Background(), params)
	if err != nil {
		return nil, "", err
	}
	o.logger.Info("history orders fetched")

	return res.GetList(), string(res.GetPagination().GetNextKey()), nil
}
func (o *History) getHistoryQueryParams(marketId string, limit uint64, key string, reverse bool) *types.QueryMarketHistoryRequest {
	res := types.QueryMarketHistoryRequest{
		Market: marketId,
		Pagination: &query.PageRequest{
			Limit:      limit,
			Reverse:    reverse,
			CountTotal: false,
		},
	}

	if key != "" {
		res.Pagination.Key = []byte(key)
	}

	return &res
}

func (o *History) GetFirstMarketOrderTime(marketId string) (time.Time, error) {
	qc, err := o.provider.GetTradebinQueryClient()
	if err != nil {
		return time.Time{}, err
	}

	params := o.getHistoryQueryParams(marketId, 1, "", false)
	o.logger.Info("fetching first market order from blockchain")
	o.logger.WithField("params", params).Info("using params to get market history")

	res, err := qc.MarketHistory(context.Background(), params)
	if err != nil {
		return time.Time{}, err
	}
	o.logger.Info("history orders fetched")

	if len(res.GetList()) == 0 {
		return time.Time{}, nil
	}

	return time.Unix(res.GetList()[0].GetExecutedAt(), 0), nil
}
