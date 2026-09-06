package interval

import (
	"sort"
	"testing"
	"time"
)

// sortedIntervals makes assertions on a group's output stable: GetIntervals
// walks a map, so the returned order is random by construction.
func sortedIntervals(intervals []*Interval) []*Interval {
	sorted := make([]*Interval, len(intervals))
	copy(sorted, intervals)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a].Start.Before(sorted[b].Start) })

	return sorted
}

func TestNewDurationGroup(t *testing.T) {
	g := NewDurationGroup(quarterHour)

	if g.Duration != quarterHour {
		t.Fatalf("expected the duration to be carried over, got %d", g.Duration)
	}

	if g.Intervals == nil {
		t.Fatal("expected an initialised interval map")
	}

	if len(g.GetIntervals()) != 0 {
		t.Fatalf("expected an empty group, got %d intervals", len(g.GetIntervals()))
	}
}

func TestGroupPutsTradesOfOneBucketIntoOneInterval(t *testing.T) {
	g := NewDurationGroup(fiveMinutes)

	// all three inside [0, 300)
	g.AddOrder(testOrder("1", "10", "10", 0))
	g.AddOrder(testOrder("3", "10", "30", 100))
	g.AddOrder(testOrder("2", "10", "20", 299))

	intervals := g.GetIntervals()
	if len(intervals) != 1 {
		t.Fatalf("expected a single interval, got %d", len(intervals))
	}

	i := intervals[0]
	if i.Start.Unix() != 0 || i.End.Unix() != 300 {
		t.Fatalf("unexpected bucket [%d, %d)", i.Start.Unix(), i.End.Unix())
	}

	if i.Duration != fiveMinutes {
		t.Fatalf("expected the group's duration on the interval, got %d", i.Duration)
	}

	assertDec(t, "open", i.OpenPrice, "1")
	assertDec(t, "close", i.ClosePrice, "2")
	assertDec(t, "highest", i.HighestPrice, "3")
	assertDec(t, "base volume", i.BaseVolume, "30")
}

func TestGroupSplitsTradesAcrossConsecutiveBuckets(t *testing.T) {
	g := NewDurationGroup(fiveMinutes)

	g.AddOrder(testOrder("1", "1", "1", 10))  // [0, 300)
	g.AddOrder(testOrder("2", "1", "2", 350)) // [300, 600)
	g.AddOrder(testOrder("3", "1", "3", 610)) // [600, 900)
	g.AddOrder(testOrder("4", "1", "4", 899)) // [600, 900), same bucket as above

	intervals := sortedIntervals(g.GetIntervals())
	if len(intervals) != 3 {
		t.Fatalf("expected three intervals, got %d", len(intervals))
	}

	wantStarts := []int64{0, 300, 600}
	for n, i := range intervals {
		if i.Start.Unix() != wantStarts[n] {
			t.Fatalf("interval %d: expected start %d, got %d", n, wantStarts[n], i.Start.Unix())
		}

		if i.End.Unix() != wantStarts[n]+300 {
			t.Fatalf("interval %d: expected end %d, got %d", n, wantStarts[n]+300, i.End.Unix())
		}
	}

	// the last bucket holds both of its trades
	assertDec(t, "last bucket open", intervals[2].OpenPrice, "3")
	assertDec(t, "last bucket close", intervals[2].ClosePrice, "4")
}

// TestGroupEmitsNoCandleForAnEmptyBucket is the behaviour the intervals endpoint
// depends on: a bucket with no trades produces NO interval at all, rather than a
// flat zero-volume candle. Gaps in a chart are therefore genuine gaps in the
// stored candles, and any gap filling has to happen on the read side.
func TestGroupEmitsNoCandleForAnEmptyBucket(t *testing.T) {
	g := NewDurationGroup(fiveMinutes)

	g.AddOrder(testOrder("1", "1", "1", 100)) // [0, 300)
	g.AddOrder(testOrder("3", "1", "3", 900)) // [900, 1200), skipping two buckets

	intervals := sortedIntervals(g.GetIntervals())
	if len(intervals) != 2 {
		t.Fatalf("expected only the two populated buckets, got %d", len(intervals))
	}

	if intervals[0].Start.Unix() != 0 || intervals[1].Start.Unix() != 900 {
		t.Fatalf("unexpected buckets: %d and %d", intervals[0].Start.Unix(), intervals[1].Start.Unix())
	}
}

// TestGroupGetOrderIntervalReusesTheSameInterval checks the lookup directly: a
// second trade in a bucket must land on the pointer created for the first, not
// on a fresh interval that would reset the candle.
func TestGroupGetOrderIntervalReusesTheSameInterval(t *testing.T) {
	g := NewDurationGroup(oneHour)

	first := g.getOrderInterval(testOrder("1", "1", "1", 1732010000))
	second := g.getOrderInterval(testOrder("2", "1", "2", 1732010001))

	if first != second {
		t.Fatal("expected the same interval pointer for two trades of one bucket")
	}

	other := g.getOrderInterval(testOrder("3", "1", "3", 1732010000+3600))
	if other == first {
		t.Fatal("expected a different interval for the next bucket")
	}

	if len(g.Intervals) != 2 {
		t.Fatalf("expected two intervals to be tracked, got %d", len(g.Intervals))
	}
}

func TestGroupGetTimestampIntervalUsesTheGroupDuration(t *testing.T) {
	for _, duration := range []Length{fiveMinutes, quarterHour, oneHour, fourHours, oneDay} {
		g := NewDurationGroup(duration)

		start, end := g.getTimestampInterval(1732010000)
		wantStart, wantEnd := GetTimestampInterval(1732010000, duration)

		if !start.Equal(wantStart) || !end.Equal(wantEnd) {
			t.Fatalf("duration %d: expected [%s, %s), got [%s, %s)", duration, wantStart, wantEnd, start, end)
		}
	}
}

// TestGroupIntervalsAreKeyedByTheBucketStart guards the map key: the key has to
// be exactly the start the interval carries, otherwise the second trade of a
// bucket would open a new candle.
func TestGroupIntervalsAreKeyedByTheBucketStart(t *testing.T) {
	g := NewDurationGroup(oneDay)
	g.AddOrder(testOrder("1", "1", "1", 1732010000))

	for key, i := range g.Intervals {
		if !key.Equal(i.Start) {
			t.Fatalf("expected the key %s to match the interval start %s", key, i.Start)
		}

		if key.Location() != time.Local {
			t.Fatalf("expected keys in the local zone, got %s", key.Location())
		}
	}
}
