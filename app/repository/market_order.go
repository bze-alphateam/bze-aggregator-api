package repository

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/jmoiron/sqlx"
	"log"
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

// Upsert deletes all orders for the provided marketIds and inserts the newly retrieved list.
// in case of failure it rolls back the sql transaction
func (r *MarketOrderRepository) Upsert(list []*entity.MarketOrder, marketIds []string) error {
	tx, err := r.db.Beginx()
	if err != nil {
		log.Fatalln(err)
	}
	defer tx.Rollback()

	// Delete statement
	deleteQ, deleteArgs, err := sqlx.In("DELETE FROM market_order WHERE market_id IN (?)", marketIds)
	if err != nil {
		return err
	}
	deleteQ = tx.Rebind(deleteQ)
	_, err = tx.Exec(deleteQ, deleteArgs...)
	if err != nil {
		return err
	}

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
