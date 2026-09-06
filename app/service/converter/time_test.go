package converter

import (
	"testing"
	"time"
)

func TestMillisecondsToTime(t *testing.T) {
	// BZE-113 regression: cached block times used to be truncated to whole
	// seconds, which collapsed several blocks onto the same timestamp
	t.Run("milliseconds survive the conversion", func(t *testing.T) {
		const ms = int64(1725000000123)

		got := MillisecondsToTime(ms)
		if got.UnixMilli() != ms {
			t.Fatalf("expected %d milliseconds back, got %d", ms, got.UnixMilli())
		}

		if got.Nanosecond() != 123000000 {
			t.Fatalf("expected 123ms of sub-second precision, got %dns", got.Nanosecond())
		}

		if !got.UTC().Equal(time.Date(2024, 8, 30, 6, 40, 0, 123000000, time.UTC)) {
			t.Fatalf("unexpected instant: %s", got.UTC())
		}
	})

	t.Run("whole seconds keep no sub-second part", func(t *testing.T) {
		got := MillisecondsToTime(1725000000000)
		if got.Nanosecond() != 0 {
			t.Fatalf("expected no sub-second part, got %dns", got.Nanosecond())
		}
	})

	// time.Unix returns a local time; callers that need UTC have to convert
	t.Run("the result is in the local location", func(t *testing.T) {
		if loc := MillisecondsToTime(1725000000123).Location(); loc != time.Local {
			t.Fatalf("expected the local location, got %s", loc)
		}
	})

	t.Run("non-positive input yields the zero time", func(t *testing.T) {
		for _, ms := range []int64{0, -1, -1725000000123} {
			if got := MillisecondsToTime(ms); !got.IsZero() {
				t.Fatalf("expected the zero time for %d, got %s", ms, got)
			}
		}
	})
}
