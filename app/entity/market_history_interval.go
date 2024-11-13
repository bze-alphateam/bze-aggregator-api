package entity

import "time"

type MarketHistoryInterval struct {
	ID           int        `db:"id"`
	MarketID     string     `db:"market_id"`
	Length       int        `db:"length"`
	StartAt      time.Time  `db:"start_at"`
	EndAt        time.Time  `db:"end_at"`
	LowestPrice  string     `db:"lowest_price"`
	OpenPrice    string     `db:"open_price"`
	AveragePrice string     `db:"average_price"`
	HighestPrice string     `db:"highest_price"`
	ClosePrice   string     `db:"close_price"`
	BaseVolume   string     `db:"base_volume"`
	QuoteVolume  string     `db:"quote_volume"`
	CreatedAt    time.Time  `db:"i_created_at"`
	UpdatedAt    *time.Time `db:"i_updated_at"`
}
