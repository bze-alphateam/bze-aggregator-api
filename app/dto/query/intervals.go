package query

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"time"
)

type IntervalsParams struct {
	MarketId string
	Length   int
	Limit    int
	StartAt  time.Time
}

type IntervalsMap map[int64]entity.MarketHistoryInterval

func (m IntervalsMap) Elements() []entity.MarketHistoryInterval {
	var result []entity.MarketHistoryInterval
	if m == nil {
		return result
	}

	for _, i := range m {
		result = append(result, i)
	}

	return result
}

type TradingIntervalsMap map[int64]entity.TradingViewInterval

func (m TradingIntervalsMap) Elements() []entity.TradingViewInterval {
	var result []entity.TradingViewInterval
	if m == nil {
		return result
	}

	for _, i := range m {
		result = append(result, i)
	}

	return result
}
