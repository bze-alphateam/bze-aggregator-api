package entity

import "time"

type MarketHistoryInterval struct {
	ID           int        `db:"id" json:"-"`
	MarketID     string     `db:"market_id" json:"market_id"`
	Length       int        `db:"length" json:"minutes"`
	StartAt      time.Time  `db:"start_at" json:"start_at"`
	EndAt        time.Time  `db:"end_at" json:"end_at"`
	LowestPrice  string     `db:"lowest_price" json:"lowest_price"`
	OpenPrice    string     `db:"open_price" json:"open_price"`
	AveragePrice string     `db:"average_price" json:"average_price"`
	HighestPrice string     `db:"highest_price" json:"highest_price"`
	ClosePrice   string     `db:"close_price" json:"close_price"`
	BaseVolume   string     `db:"base_volume" json:"base_volume"`
	QuoteVolume  string     `db:"quote_volume" json:"quote_volume"`
	CreatedAt    time.Time  `db:"i_created_at" json:"-"`
	UpdatedAt    *time.Time `db:"i_updated_at" json:"-"`
}
