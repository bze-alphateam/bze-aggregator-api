package repository

import (
	"database/sql"
	"errors"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
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

func (r MarketHistoryRepository) GetLastHistoryOrder(marketId string) (*entity.MarketHistory, error) {
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

func (r MarketHistoryRepository) SaveMarketHistoryOrders(marketId string, list []*entity.MarketHistory, clearExecutedAt []time.Time) error {
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
