package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto/query"
)

func TestGetIntervalsByExecutedAt_TruncatesTimeParam(t *testing.T) {
	db := &fakeDB{}
	repo, err := NewMarketIntervalRepository(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err = repo.GetIntervalsByExecutedAt("ubze/uvdl", timeWithNanos, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := boundTime(t, db.args, 2)
	assertNoSubMicros(t, got)
	if want := timeWithNanos.Truncate(time.Microsecond); !got.Equal(want) {
		t.Fatalf("expected bound time %s, got %s", want.Format(time.RFC3339Nano), got.Format(time.RFC3339Nano))
	}
}

func TestGetIntervalsBy_TruncatesStartAtParam(t *testing.T) {
	db := &fakeDB{}
	repo, err := NewMarketIntervalRepository(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = repo.GetIntervalsBy(&query.IntervalsParams{MarketId: "ubze/uvdl", Length: 5, StartAt: timeWithNanos, Limit: 10})
	if !errors.Is(err, errFakeQuery) {
		t.Fatalf("expected the fake query error, got %v", err)
	}

	got := boundTime(t, db.args, 2)
	assertNoSubMicros(t, got)
	if want := timeWithNanos.Truncate(time.Microsecond); !got.Equal(want) {
		t.Fatalf("expected bound time %s, got %s", want.Format(time.RFC3339Nano), got.Format(time.RFC3339Nano))
	}
}
