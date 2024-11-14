package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/query"
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

func (r *MarketIntervalRepository) GetIntervalsBy(params *query.IntervalsParams) (query.IntervalsMap, error) {
	if params.MarketId == "" {
		return nil, fmt.Errorf("can not get intervals without market_id")
	}

	if params.Length == 0 {
		return nil, fmt.Errorf("can not get intervals without length")
	}

	args := []interface{}{params.MarketId, params.Length}
	q := `
		SELECT * FROM market_history_interval mhi
		WHERE mhi.market_id = ?
		AND mhi.length = ?
`
	if !params.StartAt.Equal(time.Time{}) {
		q = fmt.Sprintf("%s AND mhi.start_at >= ?", q)
		args = append(args, params.StartAt)
	}

	q = fmt.Sprintf("%s ORDER BY start_at DESC", q)
	if params.Limit > 0 {
		q = fmt.Sprintf("%s LIMIT %d", q, params.Limit)
	}

	rows, err := r.db.Queryx(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(query.IntervalsMap)
	for rows.Next() {
		var e entity.MarketHistoryInterval
		if err = rows.StructScan(&e); err != nil {
			return nil, err
		}

		res[e.StartAt.Unix()] = e
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return res, nil
}
