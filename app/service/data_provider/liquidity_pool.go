package data_provider

import (
	"context"
	"fmt"

	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/sirupsen/logrus"
)

type LiquidityPool struct {
	provider clientProvider
	logger   logrus.FieldLogger
}

func NewLiquidityPoolProvider(cl clientProvider, logger logrus.FieldLogger) (*LiquidityPool, error) {
	if cl == nil || logger == nil {
		return nil, fmt.Errorf("invalid dependencies provided to NewLiquidityPoolProvider")
	}

	return &LiquidityPool{
		provider: cl,
		logger:   logger.WithField("service", "DataProvider.LiquidityPool"),
	}, nil
}

func (lp *LiquidityPool) GetAllLiquidityPools() ([]tradebinTypes.LiquidityPool, error) {
	lp.logger.Info("getting tradebin query client")
	qc, err := lp.provider.GetTradebinQueryClient()
	if err != nil {
		return nil, err
	}

	params := lp.getLiquidityPoolsParams()
	lp.logger.Info("fetching liquidity pools from blockchain")

	res, err := qc.AllLiquidityPools(context.Background(), params)
	if err != nil {
		return nil, err
	}
	lp.logger.Info("liquidity pools fetched")

	return res.GetList(), nil
}

func (lp *LiquidityPool) GetLiquidityPool(poolId string) (*tradebinTypes.LiquidityPool, error) {
	lp.logger.Infof("getting tradebin query client for pool %s", poolId)
	qc, err := lp.provider.GetTradebinQueryClient()
	if err != nil {
		return nil, err
	}

	params := &tradebinTypes.QueryLiquidityPoolRequest{
		PoolId: poolId,
	}
	lp.logger.Infof("fetching liquidity pool %s from blockchain", poolId)

	res, err := qc.LiquidityPool(context.Background(), params)
	if err != nil {
		return nil, err
	}
	lp.logger.Info("liquidity pool fetched")

	return res.Pool, nil
}

func (lp *LiquidityPool) getLiquidityPoolsParams() *tradebinTypes.QueryAllLiquidityPoolsRequest {
	return &tradebinTypes.QueryAllLiquidityPoolsRequest{
		Pagination: &query.PageRequest{
			Limit: 10000,
		},
	}
}
