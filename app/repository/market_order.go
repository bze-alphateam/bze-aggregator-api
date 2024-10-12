package repository

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
)

type MarketOrderRepository struct {
	db internal.Database
}

func NewMarketOrderRepository(db internal.Database) (*MarketOrderRepository, error) {
	if db == nil {
		return nil, internal.NewInvalidDependenciesErr("NewMarketOrderRepository")
	}

	return &MarketOrderRepository{db: db}, nil
}

func (r *MarketOrderRepository) Upsert(list []*entity.MarketOrder) error {
	query := `
	INSERT INTO market_order (
		market_id, order_type, amount, price, i_quote_amount, i_created_at
	) VALUES (
		:market_id, :order_type, :amount, :price, :i_quote_amount, NOW()
	) 
	ON DUPLICATE KEY UPDATE 
		amount=VALUES(amount),
		price=VALUES(price),
		i_quote_amount=VALUES(i_quote_amount),
		i_created_at=VALUES(i_created_at);
`

	_, err := r.db.NamedExec(query, list)
	if err != nil {
		return err
	}

	return nil
}
