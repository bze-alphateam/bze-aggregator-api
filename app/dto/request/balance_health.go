package request

import "github.com/labstack/echo/v4"

type AddressBalanceParams struct {
	Address   string `json:"address"`
	MinAmount int64  `json:"min_amount"`
	Denom     string `json:"denom"`
}

type BalanceHealthParams struct {
	Addresses []AddressBalanceParams `json:"addresses"`
}

func NewBalanceHealthParams(ctx echo.Context) (*BalanceHealthParams, error) {
	params := &BalanceHealthParams{}
	if err := ctx.Bind(params); err != nil {
		return nil, err
	}

	return params, nil
}
