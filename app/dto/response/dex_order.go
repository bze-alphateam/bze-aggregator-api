package response

import (
	"time"
)

type CoingeckoOrders struct {
	TickerId  string     `json:"ticker_id"`
	Timestamp string     `json:"timestamp"`
	Bids      [][]string `json:"bids"`
	Asks      [][]string `json:"asks"`
}

func (c *CoingeckoOrders) AddBid(price, volume string) {
	c.Bids = append(c.Bids, []string{price, volume})
}

func (c *CoingeckoOrders) AddAsk(price, volume string) {
	c.Asks = append(c.Asks, []string{price, volume})
}

func (c *CoingeckoOrders) SetTime(t time.Time) {
	c.Timestamp = t.Format("2006-01-02 15:04:05")
}

type OrdersBidAsk struct {
	Price  string `json:"price"`
	Volume string `json:"volume"`
}

type Orders struct {
	MarketId  string         `json:"market_id"`
	Timestamp string         `json:"timestamp"`
	Bids      []OrdersBidAsk `json:"bids"`
	Asks      []OrdersBidAsk `json:"asks"`
}

func (o *Orders) SetTime(t time.Time) {
	o.Timestamp = t.Format("2006-01-02 15:04:05")
}

func (o *Orders) AddBid(price, volume string) {
	o.Bids = append(o.Bids, OrdersBidAsk{price, volume})
}

func (o *Orders) AddAsk(price, volume string) {
	o.Asks = append(o.Asks, OrdersBidAsk{price, volume})
}
