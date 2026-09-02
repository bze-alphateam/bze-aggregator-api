package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/request"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/jmoiron/sqlx"
	"log"
	"time"
)

type MarketHistoryRepository struct {
	db internal.Database
}

func NewMarketHistoryRepository(db internal.Database) (*MarketHistoryRepository, error) {
	if db == nil {
		return nil, internal.NewInvalidDependenciesErr("MarketHistoryRepository")
	}

	return &MarketHistoryRepository{db: db}, nil
}

func (r *MarketHistoryRepository) SaveMarketHistory(items []*entity.MarketHistory) error {
	query := `
	INSERT INTO market_history (
		market_id, order_type, amount, price, executed_at, maker, taker, i_quote_amount, i_created_at
	) VALUES (
		:market_id, :order_type, :amount, :price, :executed_at, :maker, :taker, :i_quote_amount, NOW()
	);
`

	_, err := r.db.NamedExec(query, items)
	if err != nil {
		return err
	}

	return nil
}

func (r *MarketHistoryRepository) GetLastHistoryOrder(marketId string) (*entity.MarketHistory, error) {
	ent := entity.MarketHistory{}
	query := `SELECT * FROM market_history WHERE market_id = ? ORDER BY executed_at DESC LIMIT 1`

	err := r.db.Get(&ent, query, marketId)
	if err == nil {
		return &ent, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	return nil, err
}

func (r *MarketHistoryRepository) SaveMarketHistoryOrders(marketId string, list []*entity.MarketHistory, clearExecutedAt []time.Time) error {
	tx, err := r.db.Beginx()
	if err != nil {
		log.Fatalln(err)
	}
	defer tx.Rollback()

	// Delete statement
	if len(clearExecutedAt) > 0 {
		deleteQ, deleteArgs, err := sqlx.In("DELETE FROM market_history WHERE market_id = ? AND executed_at in (?)", marketId, clearExecutedAt)
		if err != nil {
			return err
		}
		deleteQ = tx.Rebind(deleteQ)

		_, err = tx.Exec(deleteQ, deleteArgs...)
		if err != nil {
			return err
		}
	}

	query := `
	INSERT INTO market_history (
		market_id, order_type, amount, price,  executed_at, maker, taker,  i_quote_amount, i_created_at
	) VALUES (
		:market_id, :order_type, :amount, :price, :executed_at, :maker, :taker, :i_quote_amount, NOW()
	);
`

	_, err = tx.NamedExec(query, list)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (r *MarketHistoryRepository) GetByExecutedAt(marketId string, executedAt time.Time) ([]entity.MarketHistory, error) {
	query := `SELECT * FROM market_history WHERE market_id = ? AND executed_at >= ? ORDER BY executed_at ASC LIMIT 50000`

	var results []entity.MarketHistory
	err := r.db.Select(&results, query, marketId, executedAt)
	if err == nil {
		return results, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return results, nil
	}

	return nil, err
}

func (r *MarketHistoryRepository) GetOldestNotAddedToInterval(marketId string) (*entity.MarketHistory, error) {
	ent := entity.MarketHistory{}
	query := `SELECT * FROM market_history WHERE market_id = ? AND i_added_to_interval = 0 ORDER BY executed_at ASC LIMIT 1`

	err := r.db.Get(&ent, query, marketId)
	if err == nil {
		return &ent, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	return nil, err
}

func (r *MarketHistoryRepository) MarkAsAddedToInterval(ids []int) error {
	query := "UPDATE market_history SET i_added_to_interval = 1 WHERE id IN (?)"
	query, args, err := sqlx.In(query, ids)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(query, args...)

	return err
}

func (r *MarketHistoryRepository) GetHistoryBy(params request.HistoryParams) ([]entity.MarketHistory, error) {
	query := "SELECT * FROM market_history WHERE 1 = 1"

	var args []interface{}
	if len(params.MarketId) > 0 {
		query = query + " AND market_id = ?"
		args = append(args, params.MarketId)
	}

	if params.OrderType == entity.OrderTypeBuy || params.OrderType == entity.OrderTypeSell {
		query = fmt.Sprintf("%s AND order_type = ?", query)
		args = append(args, params.OrderType)
	}

	if params.StartTime > 0 && params.EndTime > 0 {
		query = fmt.Sprintf("%s AND executed_at BETWEEN ? AND ?", query)
		args = append(args, converter.MillisecondsToTime(params.StartTime), converter.MillisecondsToTime(params.EndTime))
	}

	if len(params.Address) > 0 {
		query = fmt.Sprintf("%s AND (maker = ? OR taker = ?)", query)
		args = append(args, params.Address, params.Address)
	}

	query = fmt.Sprintf("%s ORDER BY executed_at DESC", query)

	if params.Limit > 0 {
		query = fmt.Sprintf("%s LIMIT %d", query, params.Limit)
	}

	var results []entity.MarketHistory
	err := r.db.Select(&results, query, args...)
	if err == nil {
		return results, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return results, nil
	}

	return nil, err
}

func (r *MarketHistoryRepository) GetFirstMarketOrderTime(marketId string) (time.Time, error) {
	ent := entity.MarketHistory{}
	query := `SELECT * FROM market_history WHERE market_id = ? ORDER BY executed_at ASC LIMIT 1`

	err := r.db.Get(&ent, query, marketId)
	if err == nil {
		return ent.ExecutedAt, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}

	return time.Time{}, err
}

func (r *MarketHistoryRepository) GetAddressSwapHistory(address string) ([]entity.MarketHistory, error) {
	query := `SELECT mh.* FROM market_history mh
         JOIN market_liquidity_data mld ON mh.market_id = mld.market_id
         WHERE (mh.maker = ? OR mh.taker = ?)
		ORDER BY mh.executed_at DESC
		LIMIT 100
`
	var results []entity.MarketHistory
	err := r.db.Select(&results, query, address, address)
	if err == nil {
		return results, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return results, nil
	}

	return nil, err
}
