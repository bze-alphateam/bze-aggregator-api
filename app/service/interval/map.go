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
	minCol := NewDurationGroup(minute)
	c[minCol.Duration] = minCol

	fiveCol := NewDurationGroup(fiveMinutes)
	c[fiveCol.Duration] = fiveCol

	hourCol := NewDurationGroup(oneHour)
	c[hourCol.Duration] = hourCol

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
	all = append(all, m.Collection[minute].GetIntervals()...)
	all = append(all, m.Collection[fiveMinutes].GetIntervals()...)
	all = append(all, m.Collection[oneHour].GetIntervals()...)

	return all
}
