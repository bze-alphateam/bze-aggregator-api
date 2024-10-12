package repository

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
)

type MarketRepository struct {
	db internal.Database
}

func NewMarketRepository(db internal.Database) (*MarketRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("invalid dependencies provided to NewMarketRepository")
	}

	return &MarketRepository{db: db}, nil
}

func (r *MarketRepository) SaveIfNotExists(items []entity.Market) error {
	query := `
	INSERT INTO market (
		market_id, base, quote, created_by, i_created_at
	) VALUES (
		:market_id, :base, :quote, :created_by, NOW()
	) 
	ON DUPLICATE KEY UPDATE 
		market_id=market_id;`

	_, err := r.db.NamedExec(query, items)
	if err != nil {
		return err
	}

	return nil
}
