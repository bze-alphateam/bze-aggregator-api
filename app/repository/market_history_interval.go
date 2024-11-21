package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/query"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/jmoiron/sqlx"
	"strings"
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
		ORDER BY mhi.start_at ASC
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
	rows, err := r.intervalsByRows(params, []string{"*"})
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

func (r *MarketIntervalRepository) GetTradingViewIntervalsBy(params *query.IntervalsParams) (query.TradingIntervalsMap, error) {
	var rows *sqlx.Rows
	var err error
	if params.Length > 60 {
		rows, err = r.getTradingViewIntervalsGroupedByDayRows(params)
	} else {
		rows, err = r.intervalsByRows(params, []string{"start_at", "lowest_price", "open_price", "highest_price", "close_price", "base_volume"})
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(query.TradingIntervalsMap)
	for rows.Next() {
		var e entity.TradingViewInterval
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

func (r *MarketIntervalRepository) intervalsByRows(params *query.IntervalsParams, selectFields []string) (*sqlx.Rows, error) {
	if params.MarketId == "" {
		return nil, fmt.Errorf("can not get intervals without market_id")
	}

	if params.Length == 0 {
		return nil, fmt.Errorf("can not get intervals without length")
	}

	args := []interface{}{params.MarketId, params.Length}
	q := fmt.Sprintf(`
		SELECT %s FROM market_history_interval mhi
		WHERE mhi.market_id = ?
		AND mhi.length = ?
`, strings.Join(selectFields, ","))

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

	return rows, nil
}

func (r *MarketIntervalRepository) getTradingViewIntervalsGroupedByDayRows(params *query.IntervalsParams) (*sqlx.Rows, error) {
	query := `
		SELECT
			DATE(start_at) AS start_at,
			MIN(CAST(lowest_price AS DECIMAL(18, 8))) AS lowest_price, -- Minimum lowest price
			MAX(CAST(highest_price AS DECIMAL(18, 8))) AS highest_price, -- Maximum highest price
			(SELECT CAST(open_price AS DECIMAL(18, 8))
			 FROM market_history_interval
			 WHERE market_id = ?
			   AND length = 60
			   AND DATE(start_at) = DATE(m.start_at)
			 ORDER BY start_at ASC
			 LIMIT 1) AS open_price, -- First open price of the day
			(SELECT CAST(close_price AS DECIMAL(18, 8))
			 FROM market_history_interval
			 WHERE market_id = ?
			   AND length = 60
			   AND DATE(start_at) = DATE(m.start_at)
			 ORDER BY start_at DESC
			 LIMIT 1) AS close_price, -- Last close price of the day
			SUM(CAST(base_volume AS DECIMAL(18, 8))) AS base_volume -- Total base volume
		FROM
			market_history_interval m
		WHERE
			m.market_id = ?
		  AND length = 60 -- Ensure we only select 60-minute intervals
		  AND start_at >= ? -- Replace with your start date
		GROUP BY
			DATE(start_at)
		ORDER BY
			start_at ASC;
`

	rows, err := r.db.Queryx(query, params.MarketId, params.MarketId, params.MarketId, params.StartAt.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}

	return rows, nil
}
