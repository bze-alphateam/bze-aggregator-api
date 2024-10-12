package entity

import "time"

type MarketOrder struct {
	ID          int       `db:"id"`
	MarketID    string    `db:"market_id"`
	OrderType   string    `db:"order_type"`
	Amount      uint64    `db:"amount"`
	Price       string    `db:"price"`
	QuoteAmount uint64    `db:"i_quote_amount"`
	CreatedAt   time.Time `db:"i_created_at"`
}
