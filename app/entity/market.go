package entity

import (
	"database/sql"
	"time"
)

type Market struct {
	ID        int       `db:"id"`
	MarketID  string    `db:"market_id"`
	Base      string    `db:"base"`
	Quote     string    `db:"quote"`
	CreatedBy string    `db:"created_by"`
	CreatedAt time.Time `db:"i_created_at"`
}

type MarketWithLastPrice struct {
	Market

	LastPrice sql.NullString `db:"last_price"`
}
