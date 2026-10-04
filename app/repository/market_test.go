package repository

import (
	"testing"
	"time"
)

func TestGetMarketsWithLastExecuted_TruncatesTimeParam(t *testing.T) {
	db := &fakeDB{}
	repo, err := NewMarketRepository(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.now = func() time.Time { return timeWithNanos }

	if _, err = repo.GetMarketsWithLastExecuted(24); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := boundTime(t, db.args, 0)
	assertNoSubMicros(t, got)

	want := timeWithNanos.Add(-24 * time.Hour).Truncate(time.Microsecond)
	if !got.Equal(want) {
		t.Fatalf("expected bound time %s, got %s", want.Format(time.RFC3339Nano), got.Format(time.RFC3339Nano))
	}
}
