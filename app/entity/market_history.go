package entity

import "time"

type MarketHistory struct {
	ID              int       `db:"id"`
	MarketID        string    `db:"market_id"`
	OrderType       string    `db:"order_type"`
	Amount          string    `db:"amount"`
	Price           string    `db:"price"`
	ExecutedAt      time.Time `db:"executed_at"`
	Maker           string    `db:"maker"`
	Taker           string    `db:"taker"`
	QuoteAmount     string    `db:"i_quote_amount"`
	CreatedAt       time.Time `db:"i_created_at"`
	AddedToInterval bool      `db:"i_added_to_interval"`
}
