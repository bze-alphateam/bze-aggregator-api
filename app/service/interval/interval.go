package interval

import (
	"sync"
	"time"

	"cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
)

const (
	fiveMinutes Length = 5
	quarterHour Length = 15
	oneHour     Length = 60
	fourHours   Length = 240
	oneDay      Length = 1440
)

type Length int

type Interval struct {
	Duration     Length
	Start        time.Time
	End          time.Time
	LowestPrice  math.LegacyDec //
	OpenPrice    math.LegacyDec //
	AveragePrice math.LegacyDec
	HighestPrice math.LegacyDec //
	ClosePrice   math.LegacyDec //
	BaseVolume   math.LegacyDec
	QuoteVolume  math.LegacyDec

	lowestExecutedAt  time.Time
	highestExecutedAt time.Time
	mx                sync.RWMutex
}

func GetBiggestDuration() Length {
	return oneDay
}

func NewInterval(start, end time.Time, duration Length) *Interval {
	return &Interval{
		Start:        start,
		End:          end,
		Duration:     duration,
		LowestPrice:  math.LegacyZeroDec(),
		OpenPrice:    math.LegacyZeroDec(),
		AveragePrice: math.LegacyZeroDec(),
		HighestPrice: math.LegacyZeroDec(),
		ClosePrice:   math.LegacyZeroDec(),
		BaseVolume:   math.LegacyZeroDec(),
		QuoteVolume:  math.LegacyZeroDec(),
	}
}

func (i *Interval) AddOrder(o *entity.MarketHistory) {
	i.mx.Lock()
	defer i.mx.Unlock()
	price := math.LegacyMustNewDecFromStr(o.Price)

	if i.lowestExecutedAt == (time.Time{}) || i.lowestExecutedAt.After(o.ExecutedAt) {
		i.lowestExecutedAt = o.ExecutedAt
		i.OpenPrice = price
	}

	if i.highestExecutedAt == (time.Time{}) || i.highestExecutedAt.Before(o.ExecutedAt) {
		i.highestExecutedAt = o.ExecutedAt
		i.ClosePrice = price
	}

	if i.LowestPrice.IsZero() || price.LT(i.LowestPrice) {
		i.LowestPrice = price
	}

	if i.HighestPrice.IsZero() || price.GT(i.HighestPrice) {
		i.HighestPrice = price
	}

	orderBaseVolume := math.LegacyMustNewDecFromStr(o.Amount)
	orderQuoteVolume := math.LegacyMustNewDecFromStr(o.QuoteAmount)
	if i.AveragePrice.IsZero() {
		i.AveragePrice = price
		i.BaseVolume = orderBaseVolume
		i.QuoteVolume = orderQuoteVolume

		return
	}

	//calculate average price and AFTERWARDS add volumes
	newBaseVolume := orderBaseVolume.Add(i.BaseVolume)
	newQuoteVolume := orderQuoteVolume.Add(i.QuoteVolume)
	if i.HighestPrice.Equal(i.LowestPrice) {
		i.AveragePrice = price
	} else {
		newAvgPrice := newQuoteVolume.Quo(newBaseVolume)
		i.AveragePrice = newAvgPrice
	}
	i.BaseVolume = newBaseVolume
	i.QuoteVolume = newQuoteVolume
}

func GetTimestampInterval(timestamp int64, duration Length) (start time.Time, end time.Time) {
	intervalSeconds := int64(duration * 60)
	rounded := timestamp / intervalSeconds * intervalSeconds

	start = time.Unix(rounded, 0)
	end = time.Unix(rounded+intervalSeconds, 0)

	return start, end
}
