package interval

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"sync"
	"time"
)

type Group struct {
	Duration  Length
	Intervals map[time.Time]*Interval

	mx sync.RWMutex
}

func NewDurationGroup(d Length) *Group {
	return &Group{
		Intervals: make(map[time.Time]*Interval),
		Duration:  d,
	}
}

func (c *Group) AddOrder(o *entity.MarketHistory) {
	i := c.getOrderInterval(o)
	i.AddOrder(o)
}

func (c *Group) getOrderInterval(o *entity.MarketHistory) *Interval {
	c.mx.Lock()
	defer c.mx.Unlock()
	start, end := c.getTimestampInterval(o.ExecutedAt.Unix())
	exists, ok := c.Intervals[start]
	if ok {
		return exists
	}

	c.Intervals[start] = NewInterval(start, end, c.Duration)

	return c.Intervals[start]
}

func (c *Group) getTimestampInterval(timestamp int64) (start time.Time, end time.Time) {
	return GetTimestampInterval(timestamp, c.Duration)
}

func (c *Group) GetIntervals() (intervals []*Interval) {
	for _, i := range c.Intervals {
		intervals = append(intervals, i)
	}

	return intervals
}
