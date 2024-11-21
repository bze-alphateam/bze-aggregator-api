package request

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"slices"
	"strings"
)

const (
	intervalFiveMinutes = 5
	intervalQuarterHour = 15
	intervalHour        = 60
	intervalFourHours   = 240
	intervalDay         = 1440 //1 day in minutes

	defaultIntervalsLimit = 500
	maxIntervalsLimit     = 5000
)

type DexInterval struct {
	MarketId string `query:"market_id"` // ubze/uvdl
	TickerId string `query:"ticker_id"` // ubze_uvdl
	Minutes  int    `query:"minutes"`
	Limit    int    `query:"limit"`
	Format   string `query:"format"`
}

func NewDexInterval(ctx echo.Context) (*DexInterval, error) {
	params := &DexInterval{}
	if err := ctx.Bind(params); err != nil {
		return nil, err
	}

	return params, nil
}

func (i *DexInterval) Validate() error {
	allIntervals := []int{intervalFiveMinutes, intervalQuarterHour, intervalHour, intervalFourHours, intervalDay}
	if !slices.Contains(allIntervals, i.Minutes) {
		return fmt.Errorf("invalid minutes. expected: %d, %d, %d, %d, %d", intervalFiveMinutes, intervalQuarterHour, intervalHour, intervalFourHours, intervalDay)
	}

	if i.Limit <= 0 {
		i.Limit = defaultIntervalsLimit
	} else {
		if i.Minutes != intervalDay && i.Limit > maxIntervalsLimit {
			return fmt.Errorf("limit can not be greater than %d", maxIntervalsLimit)
		}
	}

	if len(i.MarketId) > 1 {
		return nil
	}

	if len(i.TickerId) > 1 {
		return nil
	}

	return fmt.Errorf("please provide market_id or ticker_id")
}

func (i *DexInterval) MustGetMarketId() string {
	if len(i.MarketId) > 0 {
		return i.MarketId
	}

	return strings.ReplaceAll(i.TickerId, "_", "/")
}

func (i *DexInterval) IsTradingViewFormat() bool {
	return i.Format == "tv"
}
