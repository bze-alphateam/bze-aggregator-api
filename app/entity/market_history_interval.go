package entity

import (
	"encoding/json"
	"time"
)

type StartAtAware interface {
	GetStartAt() time.Time
}

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

func (m *MarketHistoryInterval) GetStartAt() time.Time {
	return m.StartAt
}

type TradingViewInterval struct {
	StartAt      time.Time `db:"start_at" json:"-"`
	LowestPrice  float64   `db:"lowest_price" json:"low"`
	OpenPrice    float64   `db:"open_price" json:"open"`
	HighestPrice float64   `db:"highest_price" json:"high"`
	ClosePrice   float64   `db:"close_price" json:"close"`
	BaseVolume   float64   `db:"base_volume" json:"value"`
}

func (t *TradingViewInterval) GetStartAt() time.Time {
	return t.StartAt
}

// MarshalJSON implements the custom JSON marshalling
func (t TradingViewInterval) MarshalJSON() ([]byte, error) {
	// Create a custom struct for marshalling with Unix timestamp for StartAt
	type Alias TradingViewInterval
	return json.Marshal(&struct {
		Time int64 `json:"time"` // Overriding "StartAt" with "time" in Unix format
		*Alias
	}{
		Time:  t.StartAt.Unix(),
		Alias: (*Alias)(&t),
	})
}
