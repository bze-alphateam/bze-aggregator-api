package interval

import (
	"sync"
	"testing"
)

func TestNewIntervalsMapCoversEveryLength(t *testing.T) {
	m := NewIntervalsMap("aaa/bbb")

	if m.MarketId != "aaa/bbb" {
		t.Fatalf("expected the market id to be carried over, got %s", m.MarketId)
	}

	want := []Length{fiveMinutes, quarterHour, oneHour, fourHours, oneDay}
	if len(m.Collection) != len(want) {
		t.Fatalf("expected %d groups, got %d", len(want), len(m.Collection))
	}

	for _, duration := range want {
		group, ok := m.Collection[duration]
		if !ok {
			t.Fatalf("no group for the %d-minute length", duration)
		}

		// the map is keyed by the group's own duration: a mismatch would send
		// trades into a candle of the wrong length
		if group.Duration != duration {
			t.Fatalf("the group at key %d carries duration %d", duration, group.Duration)
		}
	}

	if len(m.GetIntervals()) != 0 {
		t.Fatalf("expected a fresh map to hold no intervals, got %d", len(m.GetIntervals()))
	}
}

func TestMapAddOrderLandsInEveryLength(t *testing.T) {
	m := NewIntervalsMap("aaa/bbb")
	m.AddOrder(testOrder("2", "10", "20", 1732010000))

	intervals := m.GetIntervals()
	if len(intervals) != 5 {
		t.Fatalf("expected one interval per length, got %d", len(intervals))
	}

	// one interval per length, each anchored on its own bucket
	wantStarts := map[Length]int64{
		fiveMinutes: 1732009800,
		quarterHour: 1732009500,
		oneHour:     1732006800,
		fourHours:   1732003200,
		oneDay:      1731974400,
	}

	seen := make(map[Length]bool, len(wantStarts))
	for _, i := range intervals {
		start, ok := wantStarts[i.Duration]
		if !ok {
			t.Fatalf("unexpected length %d", i.Duration)
		}

		if seen[i.Duration] {
			t.Fatalf("length %d produced more than one interval", i.Duration)
		}
		seen[i.Duration] = true

		if i.Start.Unix() != start {
			t.Fatalf("length %d: expected bucket start %d, got %d", i.Duration, start, i.Start.Unix())
		}

		// the same trade is the whole candle at every length
		assertDec(t, "open", i.OpenPrice, "2")
		assertDec(t, "close", i.ClosePrice, "2")
		assertDec(t, "base volume", i.BaseVolume, "10")
	}
}

// TestMapGetIntervalsReturnsTheUnionOfItsGroups: two trades an hour apart give
// twelve five-minute candles' worth of buckets only where trades exist, and a
// single candle at the lengths wide enough to hold both.
func TestMapGetIntervalsReturnsTheUnionOfItsGroups(t *testing.T) {
	m := NewIntervalsMap("aaa/bbb")

	base := int64(1732010000)
	m.AddOrder(testOrder("1", "10", "10", base))
	m.AddOrder(testOrder("3", "10", "30", base+3600))

	counts := map[Length]int{}
	for _, i := range m.GetIntervals() {
		counts[i.Duration]++
	}

	// 09:53:20 and 10:53:20 fall in different 5, 15 and 60 minute buckets, but
	// share the 4 hour and the daily one
	want := map[Length]int{fiveMinutes: 2, quarterHour: 2, oneHour: 2, fourHours: 1, oneDay: 1}
	for duration, n := range want {
		if counts[duration] != n {
			t.Fatalf("length %d: expected %d intervals, got %d", duration, n, counts[duration])
		}
	}

	total := 0
	for _, n := range counts {
		total += n
	}

	if total != len(m.GetIntervals()) {
		t.Fatalf("the union lost intervals: counted %d, got %d", total, len(m.GetIntervals()))
	}
}

func TestMapAddOrderIsSafeUnderConcurrency(t *testing.T) {
	const goroutines = 8
	const perGoroutine = 25

	m := NewIntervalsMap("aaa/bbb")

	wg := sync.WaitGroup{}
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()

			for n := 0; n < perGoroutine; n++ {
				// spread the trades over several five-minute buckets so the
				// groups have to create intervals concurrently too
				at := int64(1732010000 + (g*perGoroutine+n)*60)
				m.AddOrder(testOrder("3", "2", "6", at))
			}
		}(g)
	}
	wg.Wait()

	// NOTE: GetIntervals takes no lock, so it may only be called once the adding
	// is finished - which is what the sync command does. Reading a group while
	// another goroutine is still adding to it is not safe.
	total := goroutines * perGoroutine

	// every trade has to be accounted for somewhere: the daily candles together
	// must carry the full base volume
	sum := int64(0)
	for _, i := range m.Collection[oneDay].GetIntervals() {
		sum += i.BaseVolume.TruncateInt64()
	}

	if sum != int64(total*2) {
		t.Fatalf("expected a total base volume of %d across the daily candles, got %d", total*2, sum)
	}
}
