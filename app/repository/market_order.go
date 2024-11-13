package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/jmoiron/sqlx"
	"log"
	"slices"
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

func (r *MarketOrderRepository) GetMarketOrdersWithDepth(marketId, orderType string, limit int) ([]entity.MarketOrder, error) {
	sort := "ASC"
	if orderType == types.OrderTypeBuy {
		sort = "DESC"
	}

	query := fmt.Sprintf("SELECT * FROM market_order WHERE market_id = ? AND order_type = ? ORDER BY price_dec %s", sort)

	if limit > 0 {
		query = fmt.Sprintf("%s LIMIT %d", query, limit)
	}

	var results []entity.MarketOrder
	err := r.db.Select(&results, query, marketId, orderType)
	if err == nil {
		if orderType == types.OrderTypeBuy {
			slices.Reverse(results)
		}

		return results, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return results, nil
	}

	return nil, err
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
		market_id, order_type, amount, price, price_dec, i_quote_amount, i_created_at
	) VALUES (
		:market_id, :order_type, :amount, :price, :price_dec, :i_quote_amount, NOW()
	)`

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

func (r *MarketOrderRepository) GetHighestBuy(marketId string) (*entity.MarketOrder, error) {
	query := `
		SELECT * FROM market_order WHERE market_id = ? AND order_type = ?  ORDER BY price_dec DESC LIMIT 1;
	`

	ent := &entity.MarketOrder{}
	err := r.db.Get(ent, query, marketId, entity.OrderTypeBuy)
	if err == nil {
		return ent, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	return nil, err
}

func (r *MarketOrderRepository) GetLowestSell(marketId string) (*entity.MarketOrder, error) {
	query := `
		SELECT * FROM market_order WHERE market_id = ? AND order_type = ?  ORDER BY price_dec ASC LIMIT 1;
	`

	ent := &entity.MarketOrder{}
	err := r.db.Get(ent, query, marketId, entity.OrderTypeSell)
	if err == nil {
		return ent, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	return nil, err
}
