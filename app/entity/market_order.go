package entity

import (
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"time"
)

const (
	OrderTypeBuy  = types.OrderTypeBuy
	OrderTypeSell = types.OrderTypeSell
)

type MarketOrder struct {
	ID          int       `db:"id"`
	MarketID    string    `db:"market_id"`
	OrderType   string    `db:"order_type"`
	Amount      string    `db:"amount"`
	Price       string    `db:"price"`
	PriceDec    float64   `db:"price_dec"`
	QuoteAmount string    `db:"i_quote_amount"`
	CreatedAt   time.Time `db:"i_created_at"`
}
