package query

type Balance struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

type BalancesResponse struct {
	Balances   []Balance `json:"balances"`
	Pagination struct {
		NextKey interface{} `json:"next_key"`
		Total   string      `json:"total"`
	} `json:"pagination"`
}
