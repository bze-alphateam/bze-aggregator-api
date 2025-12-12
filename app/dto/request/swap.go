package request

import (
	"fmt"

	"github.com/labstack/echo/v4"
)

type SwapHistoryParams struct {
	Address string `query:"address"`
}

func NewSwapHistoryParams(ctx echo.Context) (*SwapHistoryParams, error) {
	p := SwapHistoryParams{}
	if err := ctx.Bind(&p); err != nil {
		return nil, err
	}

	return &p, nil
}

func (p *SwapHistoryParams) Validate() error {
	if len(p.Address) == 0 {
		return fmt.Errorf("address is required")
	}

	return nil
}
