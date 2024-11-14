package request

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"slices"
	"strings"
)

const (
	intervalMinute      = 1
	intervalFiveMinutes = 5
	intervalHour        = 60

	minuteDefault  = 60 * 2
	fiveMinDefault = 12 * 24
	hourDefault    = 24 * 7

	minuteMax  = 60 * 4
	fiveMinMax = 12 * 48
)

type DexInterval struct {
	MarketId string `query:"market_id"` // ubze/uvdl
	TickerId string `query:"ticker_id"` // ubze_uvdl
	Minutes  int    `query:"minutes"`
	Limit    int    `query:"limit"`
}

func NewDexInterval(ctx echo.Context) (*DexInterval, error) {
	params := &DexInterval{}
	if err := ctx.Bind(params); err != nil {
		return nil, err
	}

	return params, nil
}

func (i *DexInterval) Validate() error {
	if !slices.Contains([]int{intervalMinute, intervalFiveMinutes, intervalHour}, i.Minutes) {
		return fmt.Errorf("invalid minutes. expected: %d, %d or %d", intervalMinute, intervalFiveMinutes, intervalHour)
	}

	if i.Limit == 0 {
		//set a default for 1 and 5 minutes to avoid returning all the intervals
		if i.Minutes == intervalMinute {
			i.Limit = minuteDefault
		} else if i.Minutes == intervalFiveMinutes {
			i.Limit = fiveMinDefault
		} else {
			i.Limit = hourDefault
		}
	} else {
		//do not allow too many intervals
		if i.Minutes == intervalMinute && i.Limit > minuteMax {
			return fmt.Errorf("max limit exceeded for 1 minute intervals. got %d expected not more than %d", i.Limit, minuteMax)
		} else if i.Minutes == intervalFiveMinutes && i.Limit > fiveMinMax {
			return fmt.Errorf("max limit exceeded for 5 minutes intervals. got %d expected not more than %d", i.Limit, fiveMinMax)
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
