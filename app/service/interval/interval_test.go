package interval

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
)

// testOrder builds the only shape of input candle building ever sees: a stored
// trade. Only price, the two amounts and the execution time matter here.
func testOrder(price, amount, quoteAmount string, executedAtUnix int64) *entity.MarketHistory {
	return &entity.MarketHistory{
		MarketID:    "aaa/bbb",
		Price:       price,
		Amount:      amount,
		QuoteAmount: quoteAmount,
		ExecutedAt:  time.Unix(executedAtUnix, 0),
	}
}

func assertDec(t *testing.T, name string, got math.LegacyDec, want string) {
	t.Helper()

	if got.String() != math.LegacyMustNewDecFromStr(want).String() {
		t.Fatalf("%s: expected %s, got %s", name, want, got)
	}
}

func TestGetTimestampInterval(t *testing.T) {
	// 1732010000 is 2024-11-19T09:53:20Z, deliberately not on any boundary
	const middle = 1732010000

	cases := []struct {
		name     string
		ts       int64
		duration Length
		start    int64
		end      int64
	}{
		// one case per supported length, all for the same instant
		{"five minutes", middle, fiveMinutes, 1732009800, 1732010100},
		{"quarter hour", middle, quarterHour, 1732009500, 1732010400},
		{"one hour", middle, oneHour, 1732006800, 1732010400},
		{"four hours", middle, fourHours, 1732003200, 1732017600},
		{"one day", middle, oneDay, 1731974400, 1732060800},

		// a timestamp exactly on a bucket start belongs to that bucket
		{"exactly on the bucket start", 1732009800, fiveMinutes, 1732009800, 1732010100},
		// the last second before the next bucket still belongs to this one
		{"one second before the bucket end", 1732010099, fiveMinutes, 1732009800, 1732010100},
		// a timestamp exactly on a bucket end opens the NEXT bucket: buckets are
		// half-open, [start, end)
		{"exactly on the bucket end", 1732010100, fiveMinutes, 1732010100, 1732010400},

		// the epoch itself is the first bucket start
		{"zero", 0, fiveMinutes, 0, 300},
		{"first second of the epoch bucket", 299, fiveMinutes, 0, 300},

		// negative (pre-1970) timestamps: Go truncates integer division towards
		// zero, so -1 rounds UP to bucket 0 and the returned bucket does not
		// contain the timestamp. Unreachable in production - the chain's genesis
		// is decades after the epoch - but pinned so a rewrite of the maths has
		// to think about it.
		{"minus one second", -1, fiveMinutes, 0, 300},
		{"just inside the first negative bucket", -299, fiveMinutes, 0, 300},
		{"exactly one bucket before the epoch", -300, fiveMinutes, -300, 0},
		{"more than one bucket before the epoch", -301, fiveMinutes, -300, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start, end := GetTimestampInterval(c.ts, c.duration)

			if start.Unix() != c.start {
				t.Fatalf("expected start %d, got %d", c.start, start.Unix())
			}

			if end.Unix() != c.end {
				t.Fatalf("expected end %d, got %d", c.end, end.Unix())
			}

			if got := end.Sub(start); got != time.Duration(c.duration)*time.Minute {
				t.Fatalf("expected a bucket %d minutes wide, got %s", c.duration, got)
			}
		})
	}
}

// TestGetTimestampIntervalIsAnchoredToTheEpochNotTheMachineZone is the answer to
// "are buckets UTC or machine-local?": they are anchored to the Unix epoch,
// which is UTC. The bucket maths is plain integer division on epoch seconds, so
// the process timezone cannot move a boundary - CI (UTC) and production compute
// the same candles. What the zone does change is only how the boundary is
// FORMATTED (see TestGetTimestampIntervalReturnsTimesInTheMachineZone), which is
// why the API's sample response shows +02:00 offsets.
//
// The practical consequence, intended: a "daily" candle is a UTC day. In a
// +05:45 zone it starts at 05:45 local, not at local midnight.
func TestGetTimestampIntervalIsAnchoredToTheEpochNotTheMachineZone(t *testing.T) {
	original := time.Local
	defer func() { time.Local = original }()

	const ts = 1732010000

	want := map[Length][2]int64{
		fiveMinutes: {1732009800, 1732010100},
		oneDay:      {1731974400, 1732060800},
	}

	// a whole-hour zone, a negative one and a 45-minute one: none of them may
	// shift a boundary by a single second
	zones := []*time.Location{
		time.UTC,
		time.FixedZone("plus-two", 2*60*60),
		time.FixedZone("minus-five", -5*60*60),
		time.FixedZone("plus-five-fortyfive", 5*60*60+45*60),
	}

	for _, zone := range zones {
		time.Local = zone

		for duration, bounds := range want {
			start, end := GetTimestampInterval(ts, duration)
			if start.Unix() != bounds[0] || end.Unix() != bounds[1] {
				t.Fatalf("zone %s moved the %d-minute bucket: got [%d, %d), want [%d, %d)",
					zone, duration, start.Unix(), end.Unix(), bounds[0], bounds[1])
			}
		}
	}

	time.Local = time.UTC
	if start, _ := GetTimestampInterval(ts, oneDay); start.Format(time.RFC3339) != "2024-11-19T00:00:00Z" {
		t.Fatalf("a daily bucket is expected to start at UTC midnight, got %s", start.Format(time.RFC3339))
	}
}

// TestGetTimestampIntervalReturnsTimesInTheMachineZone pins the other half: the
// instant is machine independent, the time.Time carrying it is not. It is built
// with time.Unix, so it is expressed in time.Local. Two boundaries for the same
// instant in different zones are NOT interchangeable as Group map keys - only
// the fact that every key comes out of this one function keeps that consistent.
func TestGetTimestampIntervalReturnsTimesInTheMachineZone(t *testing.T) {
	original := time.Local
	defer func() { time.Local = original }()

	time.Local = time.FixedZone("plus-two", 2*60*60)

	start, end := GetTimestampInterval(1732010000, oneHour)

	if start.Location() != time.Local || end.Location() != time.Local {
		t.Fatalf("expected both boundaries in the local zone, got %s and %s", start.Location(), end.Location())
	}

	// same instant as the UTC run above, only rendered differently - this is
	// where the +02:00 offsets in the API's sample response come from
	if got := start.Format(time.RFC3339); got != "2024-11-19T11:00:00+02:00" {
		t.Fatalf("expected the boundary rendered in the local zone, got %s", got)
	}

	if start.Unix() != 1732006800 {
		t.Fatalf("the zone must not move the instant, got %d", start.Unix())
	}
}

func TestNewIntervalStartsWithZeroDecs(t *testing.T) {
	start := time.Unix(1732009800, 0)
	end := time.Unix(1732010100, 0)

	i := NewInterval(start, end, fiveMinutes)

	if !i.Start.Equal(start) || !i.End.Equal(end) {
		t.Fatalf("expected the boundaries to be carried over, got %s - %s", i.Start, i.End)
	}

	if i.Duration != fiveMinutes {
		t.Fatalf("expected the duration to be carried over, got %d", i.Duration)
	}

	// every dec must be usable without a nil check: LegacyDec is a struct around
	// a pointer and a zero value panics on arithmetic
	decs := map[string]math.LegacyDec{
		"LowestPrice":  i.LowestPrice,
		"OpenPrice":    i.OpenPrice,
		"AveragePrice": i.AveragePrice,
		"HighestPrice": i.HighestPrice,
		"ClosePrice":   i.ClosePrice,
		"BaseVolume":   i.BaseVolume,
		"QuoteVolume":  i.QuoteVolume,
	}

	for name, dec := range decs {
		if dec.IsNil() {
			t.Fatalf("%s is a nil dec", name)
		}

		if !dec.IsZero() {
			t.Fatalf("expected %s to start at zero, got %s", name, dec)
		}
	}
}

func TestAddOrderWithASingleTrade(t *testing.T) {
	i := NewInterval(time.Unix(0, 0), time.Unix(300, 0), fiveMinutes)
	i.AddOrder(testOrder("2.5", "10", "25", 10))

	// a lone trade is open, close, high and low at the same time
	assertDec(t, "open", i.OpenPrice, "2.5")
	assertDec(t, "close", i.ClosePrice, "2.5")
	assertDec(t, "highest", i.HighestPrice, "2.5")
	assertDec(t, "lowest", i.LowestPrice, "2.5")
	assertDec(t, "average", i.AveragePrice, "2.5")
	assertDec(t, "base volume", i.BaseVolume, "10")
	assertDec(t, "quote volume", i.QuoteVolume, "25")
}

// TestAddOrderOutOfChronologicalOrder is what the lowestExecutedAt /
// highestExecutedAt guards exist for: candles are rebuilt from a query whose
// order is not guaranteed, so open and close must follow executed_at, not the
// order in which trades are handed over.
func TestAddOrderOutOfChronologicalOrder(t *testing.T) {
	i := NewInterval(time.Unix(0, 0), time.Unix(300, 0), fiveMinutes)

	// the newest trade first, then the oldest, then one in between
	i.AddOrder(testOrder("3", "10", "30", 200))
	i.AddOrder(testOrder("1", "10", "10", 100))
	i.AddOrder(testOrder("7", "10", "70", 150))

	assertDec(t, "open", i.OpenPrice, "1")
	assertDec(t, "close", i.ClosePrice, "3")
	assertDec(t, "lowest", i.LowestPrice, "1")
	assertDec(t, "highest", i.HighestPrice, "7")
}

func TestAddOrderTracksHighestAndLowestPrice(t *testing.T) {
	i := NewInterval(time.Unix(0, 0), time.Unix(300, 0), fiveMinutes)

	for n, price := range []string{"5", "2", "9", "4", "0.5", "6"} {
		i.AddOrder(testOrder(price, "1", price, int64(10+n)))
	}

	assertDec(t, "lowest", i.LowestPrice, "0.5")
	assertDec(t, "highest", i.HighestPrice, "9")
	assertDec(t, "open", i.OpenPrice, "5")
	assertDec(t, "close", i.ClosePrice, "6")
}

// TestAddOrderAveragePriceIsVolumeWeighted names the definition the code uses:
// the average price of a candle is quote volume / base volume (VWAP), NOT the
// arithmetic mean of the trade prices. The two disagree here on purpose - the
// arithmetic mean of 1, 3 and 2 is 2, and so is the VWAP only because the
// weights were chosen to make the sums work out; the third trade carries twice
// the volume of the others.
func TestAddOrderAveragePriceIsVolumeWeighted(t *testing.T) {
	i := NewInterval(time.Unix(0, 0), time.Unix(300, 0), fiveMinutes)

	i.AddOrder(testOrder("1", "10", "10", 100))
	i.AddOrder(testOrder("5", "10", "50", 200))
	i.AddOrder(testOrder("2", "20", "40", 300))

	// (10 + 50 + 40) / (10 + 10 + 20) = 100 / 40 = 2.5, while the arithmetic
	// mean of the three prices would be 8/3 = 2.666...
	assertDec(t, "average", i.AveragePrice, "2.5")
	assertDec(t, "base volume", i.BaseVolume, "40")
	assertDec(t, "quote volume", i.QuoteVolume, "100")
}

// TestAddOrderAveragePriceSkipsTheDivisionWhileHighEqualsLow documents the
// shortcut: while every trade in the candle had the same price the average is
// simply that price, and the division is never performed.
func TestAddOrderAveragePriceSkipsTheDivisionWhileHighEqualsLow(t *testing.T) {
	i := NewInterval(time.Unix(0, 0), time.Unix(300, 0), fiveMinutes)

	i.AddOrder(testOrder("2", "10", "20", 100))
	i.AddOrder(testOrder("2", "5", "10", 200))

	assertDec(t, "average", i.AveragePrice, "2")
	assertDec(t, "base volume", i.BaseVolume, "15")
	assertDec(t, "quote volume", i.QuoteVolume, "30")
}

// TestAddOrderWithIdenticalTimestampsKeepsTheFirstTradeAsClose pins current
// behaviour.
//
// BUG: trades that share an executed_at - which is every trade filled in the
// same block, the normal case for an order book - are compared with strict
// After/Before, so neither the open nor the close price ever moves off the
// FIRST trade handed over. The candle's close is therefore whichever row the
// history query returned first, and that query orders by executed_at only, with
// no tie breaker, so the close price of a busy block is effectively arbitrary.
// Volumes and high/low are unaffected. Fixing it needs a tie-break rule (id
// order) rather than a one-line change, so it is filed as BZE-135.
func TestAddOrderWithIdenticalTimestampsKeepsTheFirstTradeAsClose(t *testing.T) {
	i := NewInterval(time.Unix(0, 0), time.Unix(300, 0), fiveMinutes)

	i.AddOrder(testOrder("1", "10", "10", 100))
	i.AddOrder(testOrder("2", "10", "20", 100))

	assertDec(t, "open", i.OpenPrice, "1")
	assertDec(t, "close", i.ClosePrice, "1") // would be "2" if ties moved the close
	assertDec(t, "lowest", i.LowestPrice, "1")
	assertDec(t, "highest", i.HighestPrice, "2")
	assertDec(t, "base volume", i.BaseVolume, "20")
	assertDec(t, "quote volume", i.QuoteVolume, "30")
}

// TestAddOrderDiscardsTheVolumeOfALeadingZeroPricedTrade pins current behaviour.
//
// BUG: AveragePrice being zero is used as the "this is the first trade" flag.
// A trade priced at zero leaves it zero, so the NEXT trade takes the first-trade
// branch again and OVERWRITES the volumes instead of adding to them - the zero
// priced trade's base volume silently disappears from the candle. The same guard
// on LowestPrice drops the zero from the low. Zero prices are reachable today
// (see BZE-133), which is what makes this more than theoretical. The fix is a
// real restructure - an explicit trade counter instead of the zero probes - so
// it is filed as BZE-136 rather than patched here.
func TestAddOrderDiscardsTheVolumeOfALeadingZeroPricedTrade(t *testing.T) {
	i := NewInterval(time.Unix(0, 0), time.Unix(300, 0), fiveMinutes)

	i.AddOrder(testOrder("0", "10", "0", 100))
	assertDec(t, "base volume after the zero priced trade", i.BaseVolume, "10")

	i.AddOrder(testOrder("2", "5", "10", 200))

	// the first trade's 10 base is gone instead of totalling 15
	assertDec(t, "base volume", i.BaseVolume, "5")
	assertDec(t, "quote volume", i.QuoteVolume, "10")
	// ... and the zero is no longer the low
	assertDec(t, "lowest", i.LowestPrice, "2")
	// open still points at the zero priced trade, so the candle is internally
	// inconsistent: it opens below its own low
	assertDec(t, "open", i.OpenPrice, "0")
}

// TestAddOrderPanicsWhenTheTotalBaseVolumeIsZero pins current behaviour.
//
// BUG: with high != low the average is quote volume / base volume, and the
// division is not guarded. Dust trades really do store an amount of "0" - the
// converter trims a fully rounded-away amount down to it - so two zero-amount
// trades at different prices crash candle building for the whole market. Same
// root cause and same ticket as the volume loss above (BZE-136).
func TestAddOrderPanicsWhenTheTotalBaseVolumeIsZero(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a division by zero panic")
		}
	}()

	i := NewInterval(time.Unix(0, 0), time.Unix(300, 0), fiveMinutes)
	i.AddOrder(testOrder("1", "0", "0", 100))
	i.AddOrder(testOrder("2", "0", "0", 200))
}

// TestAddOrderPanicsOnAnUnparsablePrice pins the constraint the callers carry:
// every string in market_history has to be a valid decimal, because AddOrder
// parses with MustNewDecFromStr and has no error path. The rows are written by
// the converters, which produce LegacyDec strings, so this cannot be reached
// without a corrupted database.
func TestAddOrderPanicsOnAnUnparsablePrice(t *testing.T) {
	for _, order := range []*entity.MarketHistory{
		testOrder("not a price", "1", "1", 100),
		testOrder("1", "not an amount", "1", 100),
		testOrder("1", "1", "not a quote amount", 100),
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("expected a panic for %+v", order)
				}
			}()

			NewInterval(time.Unix(0, 0), time.Unix(300, 0), fiveMinutes).AddOrder(order)
		}()
	}
}

// TestAddOrderIsSafeUnderConcurrency exercises the interval's mutex. Run with
// -race: without the lock the shared decs and the executed-at guards are a data
// race. All trades carry the same price so the result stays deterministic
// whatever order the goroutines win in.
func TestAddOrderIsSafeUnderConcurrency(t *testing.T) {
	const goroutines = 8
	const perGoroutine = 50

	i := NewInterval(time.Unix(0, 0), time.Unix(300, 0), fiveMinutes)

	wg := sync.WaitGroup{}
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()

			for n := 0; n < perGoroutine; n++ {
				i.AddOrder(testOrder("3", "2", "6", int64(g*perGoroutine+n)))
			}
		}(g)
	}
	wg.Wait()

	total := goroutines * perGoroutine
	assertDec(t, "base volume", i.BaseVolume, strconv.Itoa(total*2))
	assertDec(t, "quote volume", i.QuoteVolume, strconv.Itoa(total*6))
	assertDec(t, "average", i.AveragePrice, "3")
	assertDec(t, "open", i.OpenPrice, "3")
	assertDec(t, "close", i.ClosePrice, "3")
}

func TestGetBiggestDuration(t *testing.T) {
	if got := GetBiggestDuration(); got != oneDay {
		t.Fatalf("expected the day length, got %d", got)
	}

	if GetBiggestDuration() != 1440 {
		t.Fatalf("the biggest duration is expected to be 1440 minutes, got %d", GetBiggestDuration())
	}
}
