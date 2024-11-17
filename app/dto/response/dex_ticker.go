package response

import "fmt"

type CoingeckoTicker struct {
	TickerId    string  `json:"ticker_id"`       //market
	Base        string  `json:"base_currency"`   //market
	Quote       string  `json:"target_currency"` //market
	MarketId    string  `json:"pool_id"`         //market
	LastPrice   float64 `json:"last_price"`      //market_history  -> last executed at
	BaseVolume  float64 `json:"base_volume"`     //market_intervals -> aggregate 5 minutes intervals
	QuoteVolume float64 `json:"target_volume"`   //market_intervals -> aggregate 5 minutes intervals
	Bid         float64 `json:"bid"`             //market_order -> the highest buy
	Ask         float64 `json:"ask"`             //market_order -> the lowest sell
	High        float64 `json:"high"`            //market_history -> the highest in this interval
	Low         float64 `json:"low"`             //market_history -> the lowest in this interval
}

func (c *CoingeckoTicker) SetMarketDetails(base, quote, marketId string) {
	c.MarketId = marketId
	c.Base = base
	c.Quote = quote

	c.TickerId = fmt.Sprintf("%s_%s", base, quote)
}

func (c *CoingeckoTicker) SetLastPrice(price float64) {
	c.LastPrice = price
}

func (c *CoingeckoTicker) SetBaseVolume(baseVolume float64) {
	c.BaseVolume = baseVolume
}

func (c *CoingeckoTicker) SetQuoteVolume(quoteVolume float64) {
	c.QuoteVolume = quoteVolume
}

func (c *CoingeckoTicker) SetBid(bid float64) {
	c.Bid = bid
}

func (c *CoingeckoTicker) SetAsk(ask float64) {
	c.Ask = ask
}

func (c *CoingeckoTicker) SetHigh(high float64) {
	c.High = high
}

func (c *CoingeckoTicker) SetLow(low float64) {
	c.Low = low
}

func (c *CoingeckoTicker) SetChange(_ float32) {}

func (c *CoingeckoTicker) SetOpenPrice(_ float64) {
}

type Ticker struct {
	Base        string  `json:"base"`  // Symbol/Currency code/Contract Address of a the base cryptoasset, eg. BTC (Contract address for DEX)
	Quote       string  `json:"quote"` //
	MarketId    string  `json:"market_id"`
	LastPrice   float64 `json:"last_price"`
	BaseVolume  float64 `json:"base_volume"`
	QuoteVolume float64 `json:"quote_volume"`
	Bid         float64 `json:"bid"`
	Ask         float64 `json:"ask"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	OpenPrice   float64 `json:"open_price"`
	Change      float32 `json:"change"`
}

func (t *Ticker) SetChange(change float32) {
	t.Change = change
}

func (t *Ticker) SetOpenPrice(price float64) {
	t.OpenPrice = price
}

func (t *Ticker) SetMarketDetails(base, quote, marketId string) {
	t.MarketId = marketId
	t.Base = base
	t.Quote = quote
}

func (t *Ticker) SetLastPrice(price float64) {
	t.LastPrice = price
}

func (t *Ticker) SetBaseVolume(baseVolume float64) {
	t.BaseVolume = baseVolume
}

func (t *Ticker) SetQuoteVolume(quoteVolume float64) {
	t.QuoteVolume = quoteVolume
}

func (t *Ticker) SetBid(bid float64) {
	t.Bid = bid
}

func (t *Ticker) SetAsk(ask float64) {
	t.Ask = ask
}

func (t *Ticker) SetHigh(high float64) {
	t.High = high
}

func (t *Ticker) SetLow(low float64) {
	t.Low = low
}
