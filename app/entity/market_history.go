package entity

import "time"

type MarketHistory struct {
	ID          int       `db:"id"`
	MarketID    string    `db:"market_id"`
	OrderType   string    `db:"order_type"`
	Amount      uint64    `db:"amount"`
	Price       string    `db:"price"`
	ExecutedAt  time.Time `db:"executed_at"`
	Maker       string    `db:"maker"`
	Taker       string    `db:"taker"`
	QuoteAmount uint64    `db:"i_quote_amount"`
	CreatedAt   time.Time `db:"i_created_at"`
}
