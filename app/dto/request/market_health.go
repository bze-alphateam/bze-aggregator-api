package request

import (
	"errors"
	"github.com/labstack/echo/v4"
)

const (
	defaultMinutes = 10
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
