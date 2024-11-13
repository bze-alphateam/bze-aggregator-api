package repository

import (
	"database/sql"
	"errors"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"time"
)

type MarketRepository struct {
	db internal.Database
}

func NewMarketRepository(db internal.Database) (*MarketRepository, error) {
	if db == nil {
		return nil, internal.NewInvalidDependenciesErr("NewMarketRepository")
	}

	return &MarketRepository{db: db}, nil
}

func (r *MarketRepository) SaveIfNotExists(items []*entity.Market) error {
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

func (r *MarketRepository) GetMarketsWithLastExecuted(hours int) ([]entity.MarketWithLastPrice, error) {
	query := `
		SELECT 
		    m.id as id,
			m.market_id as market_id,
			m.base as base,
			m.quote as quote,
			m.created_by as created_by,
			m.i_created_at as i_created_at,
			mh.price as last_price
		FROM market m
		LEFT JOIN market_history mh on mh.market_id = m.market_id
			AND mh.executed_at = (
				SELECT MAX(executed_at)
				FROM market_history
				WHERE market_id = m.market_id
				AND executed_at > ?
			)
`
	executedAt := time.Now().Add(-time.Hour * time.Duration(hours))
	var results []entity.MarketWithLastPrice
	err := r.db.Select(&results, query, executedAt)
	if err == nil {
		return results, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return results, nil
	}

	return nil, err
}
