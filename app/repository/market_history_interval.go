package repository

import (
	"database/sql"
	"errors"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"time"
)

type MarketIntervalRepository struct {
	db internal.Database
}

func NewMarketIntervalRepository(db internal.Database) (*MarketIntervalRepository, error) {
	if db == nil {
		return nil, internal.NewInvalidDependenciesErr("NewMarketIntervalRepository")
	}

	return &MarketIntervalRepository{db: db}, nil
}

func (r *MarketIntervalRepository) Save(items []*entity.MarketHistoryInterval) error {
	query := `
	INSERT INTO market_history_interval (
		market_id, length, start_at, end_at, 
		lowest_price, open_price, average_price, highest_price, close_price,
		base_volume, quote_volume, i_created_at
	) VALUES (
		:market_id, :length, :start_at, :end_at,
	    :lowest_price, :open_price, :average_price, :highest_price, :close_price,
	    :base_volume, :quote_volume, NOW()
	) 
	ON DUPLICATE KEY UPDATE 
		lowest_price = VALUES(lowest_price),
		open_price = VALUES(open_price),
		average_price = VALUES(average_price),
		highest_price = VALUES(highest_price), 
		close_price = VALUES(close_price),
		base_volume = VALUES(base_volume),
		quote_volume = VALUES(quote_volume),
		i_updated_at = NOW()
	;`

	_, err := r.db.NamedExec(query, items)
	if err != nil {
		return err
	}

	return nil
}

func (r *MarketIntervalRepository) GetIntervalsByExecutedAt(marketId string, executedAt time.Time, length int) ([]entity.MarketHistoryInterval, error) {
	query := `
		SELECT * FROM market_history_interval mhi
		WHERE mhi.market_id = ?
		AND mhi.length = ?
		AND mhi.start_at >= ?
	`

	var results []entity.MarketHistoryInterval
	err := r.db.Select(&results, query, marketId, length, executedAt)
	if err == nil {
		return results, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return results, nil
	}

	return nil, err
}
