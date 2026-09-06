package converter

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/interval"
)

const intervalTestMarketId = "factory/bze13gzq40che93tgfm9kzmkpjamah5nj0j73pyhqk/uvdl/ubze"

func intervalTestOrder(price, amount, quoteAmount string, executedAtUnix int64) *entity.MarketHistory {
	return &entity.MarketHistory{
		MarketID:    intervalTestMarketId,
		Price:       price,
		Amount:      amount,
		QuoteAmount: quoteAmount,
		ExecutedAt:  time.Unix(executedAtUnix, 0),
	}
}

// entitiesByLength indexes the result so assertions do not depend on the random
// order the intervals come out of their maps in.
func entitiesByLength(entities []*entity.MarketHistoryInterval) map[int][]*entity.MarketHistoryInterval {
	byLength := make(map[int][]*entity.MarketHistoryInterval)
	for _, e := range entities {
		byLength[e.Length] = append(byLength[e.Length], e)
	}

	for _, group := range byLength {
		sort.Slice(group, func(a, b int) bool { return group[a].StartAt.Before(group[b].StartAt) })
	}

	return byLength
}

func TestIntervalMapToEntities(t *testing.T) {
	m := interval.NewIntervalsMap(intervalTestMarketId)

	// 2024-11-19T09:53:20Z and 09:55:00Z: different 5 minute buckets, the same
	// 15, 60, 240 and 1440 minute ones
	m.AddOrder(intervalTestOrder("1.25", "10", "12.5", 1732010000))
	m.AddOrder(intervalTestOrder("2", "10", "20", 1732010100))

	entities := IntervalMapToEntities(m)

	// two five-minute candles plus one at each of the other four lengths
	if len(entities) != 6 {
		t.Fatalf("expected six candles, got %d", len(entities))
	}

	byLength := entitiesByLength(entities)

	for _, e := range entities {
		if e.MarketID != intervalTestMarketId {
			t.Fatalf("expected the map's market id on every candle, got %s", e.MarketID)
		}

		if e.EndAt.Sub(e.StartAt) != time.Duration(e.Length)*time.Minute {
			t.Fatalf("length %d: the boundaries span %s", e.Length, e.EndAt.Sub(e.StartAt))
		}

		// the id and the timestamps the repository fills in are left alone
		if e.ID != 0 || !e.CreatedAt.IsZero() || e.UpdatedAt != nil {
			t.Fatalf("expected the persistence fields to be untouched, got %+v", e)
		}
	}

	if got := len(byLength[5]); got != 2 {
		t.Fatalf("expected two five-minute candles, got %d", got)
	}

	first := byLength[5][0]
	if first.StartAt.Unix() != 1732009800 || first.EndAt.Unix() != 1732010100 {
		t.Fatalf("unexpected first bucket [%d, %d)", first.StartAt.Unix(), first.EndAt.Unix())
	}

	// a single trade candle: every price is that trade's price
	if first.LowestPrice != "1.25" || first.OpenPrice != "1.25" || first.AveragePrice != "1.25" ||
		first.HighestPrice != "1.25" || first.ClosePrice != "1.25" {
		t.Fatalf("unexpected prices on the first candle: %+v", first)
	}

	if first.BaseVolume != "10" || first.QuoteVolume != "12.5" {
		t.Fatalf("unexpected volumes on the first candle: %s / %s", first.BaseVolume, first.QuoteVolume)
	}

	// the hourly candle holds both trades
	hour := byLength[60][0]
	if hour.StartAt.Unix() != 1732006800 || hour.EndAt.Unix() != 1732010400 {
		t.Fatalf("unexpected hourly bucket [%d, %d)", hour.StartAt.Unix(), hour.EndAt.Unix())
	}

	// (12.5 + 20) / (10 + 10) = 1.625
	if hour.OpenPrice != "1.25" || hour.ClosePrice != "2" || hour.LowestPrice != "1.25" ||
		hour.HighestPrice != "2" || hour.AveragePrice != "1.625" {
		t.Fatalf("unexpected prices on the hourly candle: %+v", hour)
	}

	if hour.BaseVolume != "20" || hour.QuoteVolume != "32.5" {
		t.Fatalf("unexpected volumes on the hourly candle: %s / %s", hour.BaseVolume, hour.QuoteVolume)
	}

	if len(byLength[1440]) != 1 || byLength[1440][0].StartAt.Unix() != 1731974400 {
		t.Fatalf("unexpected daily candle: %+v", byLength[1440])
	}
}

// TestIntervalMapToEntitiesTrimsTheDecimalStrings pins the string shape the
// repository writes into the VARCHAR price and volume columns: LegacyDec always
// renders 18 decimals, and they are trimmed off before storing so the API does
// not serve "2.000000000000000000".
func TestIntervalMapToEntitiesTrimsTheDecimalStrings(t *testing.T) {
	m := interval.NewIntervalsMap(intervalTestMarketId)
	m.AddOrder(intervalTestOrder("0.000001234", "1000000", "1.234", 1732010000))

	for _, e := range IntervalMapToEntities(m) {
		for name, value := range map[string]string{
			"lowest_price":  e.LowestPrice,
			"open_price":    e.OpenPrice,
			"average_price": e.AveragePrice,
			"highest_price": e.HighestPrice,
			"close_price":   e.ClosePrice,
			"base_volume":   e.BaseVolume,
			"quote_volume":  e.QuoteVolume,
		} {
			if len(value) > 80 {
				t.Fatalf("%s does not fit the VARCHAR(80) column: %q", name, value)
			}

			// only the fractional part is trimmed: "1000000" keeps its zeros,
			// "2.000000000000000000" becomes "2"
			if strings.Contains(value, ".") && strings.HasSuffix(value, "0") {
				t.Fatalf("%s kept its trailing zeros: %q", name, value)
			}

			if strings.HasSuffix(value, ".") {
				t.Fatalf("%s was left with a dangling decimal point: %q", name, value)
			}
		}

		if e.BaseVolume != "1000000" {
			t.Fatalf("expected a whole base volume to stay an integer string, got %q", e.BaseVolume)
		}

		if e.OpenPrice != "0.000001234" {
			t.Fatalf("expected the price to survive the round trip, got %q", e.OpenPrice)
		}
	}
}

// TestIntervalMapToEntitiesOnAnEmptyMap: a market whose window holds no trades
// must produce an empty slice, not a nil dereference - the sync hands the result
// straight to the repository's batch save.
func TestIntervalMapToEntitiesOnAnEmptyMap(t *testing.T) {
	entities := IntervalMapToEntities(interval.NewIntervalsMap(intervalTestMarketId))

	if entities == nil {
		t.Fatal("expected an empty slice rather than nil")
	}

	if len(entities) != 0 {
		t.Fatalf("expected no candles, got %d", len(entities))
	}
}

// TestIntervalMapToEntitiesZeroPricedCandle pins how a zero priced candle is
// stored: as the string "0", not as an empty column. Zero prices reach candle
// building today (BZE-133), so this is the shape the charts actually read.
func TestIntervalMapToEntitiesZeroPricedCandle(t *testing.T) {
	m := interval.NewIntervalsMap(intervalTestMarketId)
	m.AddOrder(intervalTestOrder("0", "10", "0", 1732010000))

	for _, e := range IntervalMapToEntities(m) {
		if e.OpenPrice != "0" || e.ClosePrice != "0" || e.AveragePrice != "0" || e.QuoteVolume != "0" {
			t.Fatalf("expected zeroes rendered as \"0\", got %+v", e)
		}

		if e.BaseVolume != "10" {
			t.Fatalf("expected the base volume to survive, got %q", e.BaseVolume)
		}
	}
}
