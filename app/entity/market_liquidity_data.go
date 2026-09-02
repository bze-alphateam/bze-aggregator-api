package entity

type MarketLiquidityData struct {
	ID           int    `db:"id"`
	MarketID     string `db:"market_id"`
	LpDenom      string `db:"lp_denom"`
	Fee          string `db:"fee"`
	ReserveBase  string `db:"reserve_base"`
	ReserveQuote string `db:"reserve_quote"`
}
