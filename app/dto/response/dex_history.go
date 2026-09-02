package response

type CoingeckoHistoryTrade struct {
	OrderId     int    `json:"trade_id"`
	Price       string `json:"price"`
	BaseVolume  string `json:"base_volume"`
	QuoteVolume string `json:"target_volume"`
	ExecutedAt  string `json:"trade_timestamp"`
	OrderType   string `json:"type"`
}

type CoingeckoHistory struct {
	Buy  []CoingeckoHistoryTrade `json:"buy,omitempty"`
	Sell []CoingeckoHistoryTrade `json:"sell,omitempty"`
}

type HistoryTrade struct {
	OrderId     int    `json:"order_id"`
	Price       string `json:"price"`
	BaseVolume  string `json:"base_volume"`
	QuoteVolume string `json:"quote_volume"`
	ExecutedAt  string `json:"executed_at"`
	OrderType   string `json:"order_type"`
	Maker       string `json:"maker"`
	Taker       string `json:"taker"`
	PoolId      string `json:"pool_id,omitempty"`
	Base        string `json:"base,omitempty"`
	Quote       string `json:"quote,omitempty"`
}
