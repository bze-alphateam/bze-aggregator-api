package request

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"strings"
)

type OrdersParams struct {
	Format   string `query:"format"`
	MarketId string `query:"market_id"` // ubze/uvdl
	TickerId string `query:"ticker_id"` // ubze_uvdl
	Depth    int    `query:"depth"`
}

func NewOrdersParams(ctx echo.Context) (*OrdersParams, error) {
	order := &OrdersParams{}
	if err := ctx.Bind(order); err != nil {
		return nil, err
	}

	setAllowedFormat(order)

	return order, nil
}

func (o *OrdersParams) Validate() error {
	if o.Depth < 0 {
		return fmt.Errorf("depth must be a positive number")
	}

	if len(o.MarketId) > 1 {
		return nil
	}

	if len(o.TickerId) > 1 {
		return nil
	}

	return fmt.Errorf("please provide market_id or ticker_id")
}

func (o *OrdersParams) SetFormat(format string) {
	o.Format = format
}

func (o *OrdersParams) GetFormat() string {
	return o.Format
}

func (o *OrdersParams) IsCoingeckoFormat() bool {
	return o.Format == formatCoingecko
}

func (o *OrdersParams) MustGetMarketId() string {
	if len(o.MarketId) > 0 {
		return o.MarketId
	}

	return strings.ReplaceAll(o.TickerId, "_", "/")
}
