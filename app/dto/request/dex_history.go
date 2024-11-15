package request

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"strings"
)

const (
	defaultLimit = 500
	maxLimit     = 1000
)

type HistoryParams struct {
	Format   string `query:"format"`
	MarketId string `query:"market_id"` // ubze/uvdl
	TickerId string `query:"ticker_id"` // ubze_uvdl

	OrderType string `query:"type"`
	Limit     int    `query:"limit"`
	StartTime int64  `query:"start_time"`
	EndTime   int64  `query:"end_time"`
	Address   string `query:"address"`
}

func NewHistoryParams(ctx echo.Context) (*HistoryParams, error) {
	params := &HistoryParams{}
	if err := ctx.Bind(params); err != nil {
		return nil, err
	}

	setAllowedFormat(params)

	if params.Limit <= 0 {
		params.Limit = defaultLimit
	} else if params.Limit > maxLimit {
		params.Limit = maxLimit
	}

	return params, nil
}

func (o *HistoryParams) Validate() error {
	if len(o.Address) > 0 {
		return nil
	}

	if len(o.MarketId) > 1 {
		return nil
	}

	if len(o.TickerId) > 1 {
		return nil
	}

	return fmt.Errorf("please provide market_id or ticker_id")
}

func (o *HistoryParams) SetFormat(format string) {
	o.Format = format
}

func (o *HistoryParams) GetFormat() string {
	return o.Format
}

func (o *HistoryParams) IsCoingeckoFormat() bool {
	return o.Format == formatCoingecko
}

func (o *HistoryParams) MustGetMarketId() string {
	if len(o.MarketId) > 0 {
		return o.MarketId
	}

	return strings.ReplaceAll(o.TickerId, "_", "/")
}
