package request

import (
	"errors"
	"github.com/labstack/echo/v4"
)

const (
	defaultMinutes = 10
	maxMinutes     = 60 * 12
)

type MarketHealthRequest struct {
	MarketId string `query:"market_id"`
	Minutes  int    `query:"minutes"`
}

func NewMarketHealthRequest(ctx echo.Context) (*MarketHealthRequest, error) {
	mhr := &MarketHealthRequest{}
	if err := ctx.Bind(mhr); err != nil {
		return nil, err
	}

	if mhr.Minutes <= 0 {
		mhr.Minutes = defaultMinutes
	}

	return mhr, nil
}

func (mhr *MarketHealthRequest) Validate() error {
	if mhr.MarketId == "" {
		return errors.New("market_id is required")
	}

	if mhr.Minutes <= 0 {
		return errors.New("invalid minutes")
	}

	return nil
}

type AggregatorHealthRequest struct {
	Minutes int `query:"minutes"`
}

func NewAggregatorHealthRequest(ctx echo.Context) (*AggregatorHealthRequest, error) {
	ahr := &AggregatorHealthRequest{}
	if err := ctx.Bind(ahr); err != nil {
		return nil, err
	}

	if ahr.Minutes <= 0 {
		ahr.Minutes = defaultMinutes
	} else if ahr.Minutes > maxMinutes {
		ahr.Minutes = maxMinutes
	}

	return ahr, nil
}
