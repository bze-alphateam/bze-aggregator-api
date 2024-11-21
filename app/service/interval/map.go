package interval

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
)

type Map struct {
	Collection map[Length]*Group

	MarketId string
}

func NewIntervalsMap(marketId string) *Map {
	c := make(map[Length]*Group)

	fiveCol := NewDurationGroup(fiveMinutes)
	c[fiveCol.Duration] = fiveCol

	quarterCol := NewDurationGroup(quarterHour)
	c[quarterCol.Duration] = quarterCol

	hourCol := NewDurationGroup(oneHour)
	c[hourCol.Duration] = hourCol

	fourCol := NewDurationGroup(fourHours)
	c[fourCol.Duration] = fourCol

	dayCol := NewDurationGroup(oneDay)
	c[dayCol.Duration] = dayCol

	return &Map{
		Collection: c,
		MarketId:   marketId,
	}
}

func (m *Map) AddOrder(o *entity.MarketHistory) {
	for _, c := range m.Collection {
		c.AddOrder(o)
	}
}

func (m *Map) GetIntervals() (all []*Interval) {
	for _, c := range m.Collection {
		all = append(all, c.GetIntervals()...)
	}

	return all
}
