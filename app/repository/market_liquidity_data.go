package repository

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
)

type MarketLiquidityDataRepository struct {
	db internal.Database
}

func NewMarketLiquidityDataRepository(db internal.Database) (*MarketLiquidityDataRepository, error) {
	if db == nil {
		return nil, internal.NewInvalidDependenciesErr("NewMarketLiquidityDataRepository")
	}

	return &MarketLiquidityDataRepository{db: db}, nil
}

func (r *MarketLiquidityDataRepository) SaveOrUpdate(items []*entity.MarketLiquidityData) error {
	query := `
	INSERT INTO market_liquidity_data (
		market_id, lp_denom, fee, reserve_base, reserve_quote
	) VALUES (
		:market_id, :lp_denom, :fee, :reserve_base, :reserve_quote
	)
	ON DUPLICATE KEY UPDATE
		fee = VALUES(fee),
		reserve_base = VALUES(reserve_base),
		reserve_quote = VALUES(reserve_quote);`

	_, err := r.db.NamedExec(query, items)
	if err != nil {
		return err
	}

	return nil
}
