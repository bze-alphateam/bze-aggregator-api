package dto

type Coin struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

type CoinPrice struct {
	Denom      string  `json:"denom"`
	Price      float64 `json:"price"`
	PriceDenom string  `json:"price_denom"`
}
