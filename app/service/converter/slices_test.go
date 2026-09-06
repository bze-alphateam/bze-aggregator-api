package converter

import (
	"testing"

	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
)

func TestSplitIntSlice(t *testing.T) {
	for _, tc := range []struct {
		name      string
		data      []int
		batchSize int
		want      [][]int
	}{
		{name: "exact multiple", data: []int{1, 2, 3, 4}, batchSize: 2, want: [][]int{{1, 2}, {3, 4}}},
		{name: "remainder in the last batch", data: []int{1, 2, 3, 4, 5}, batchSize: 2, want: [][]int{{1, 2}, {3, 4}, {5}}},
		{name: "batch larger than the input", data: []int{1, 2}, batchSize: 10, want: [][]int{{1, 2}}},
		{name: "batch size of one", data: []int{1, 2}, batchSize: 1, want: [][]int{{1}, {2}}},
		{name: "empty input", data: []int{}, batchSize: 2, want: nil},
		{name: "nil input", data: nil, batchSize: 2, want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitIntSlice(tc.data, tc.batchSize)
			if len(got) != len(tc.want) {
				t.Fatalf("expected %d batches, got %d (%v)", len(tc.want), len(got), got)
			}

			for i, batch := range got {
				if len(batch) != len(tc.want[i]) {
					t.Fatalf("batch %d: expected %v, got %v", i, tc.want[i], batch)
				}

				for j, v := range batch {
					if v != tc.want[i][j] {
						t.Fatalf("batch %d: expected %v, got %v", i, tc.want[i], batch)
					}
				}
			}
		})
	}
}

func TestSplitIntervalsSlice(t *testing.T) {
	intervals := make([]*entity.MarketHistoryInterval, 5)
	for i := range intervals {
		intervals[i] = &entity.MarketHistoryInterval{MarketID: "aaa/bbb"}
	}

	for _, tc := range []struct {
		name      string
		data      []*entity.MarketHistoryInterval
		batchSize int
		wantSizes []int
	}{
		{name: "exact multiple", data: intervals[:4], batchSize: 2, wantSizes: []int{2, 2}},
		{name: "remainder in the last batch", data: intervals, batchSize: 2, wantSizes: []int{2, 2, 1}},
		{name: "batch larger than the input", data: intervals[:2], batchSize: 10, wantSizes: []int{2}},
		{name: "empty input", data: nil, batchSize: 100, wantSizes: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitIntervalsSlice(tc.data, tc.batchSize)
			if len(got) != len(tc.wantSizes) {
				t.Fatalf("expected %d batches, got %d", len(tc.wantSizes), len(got))
			}

			total := 0
			for i, batch := range got {
				if len(batch) != tc.wantSizes[i] {
					t.Fatalf("batch %d: expected %d entries, got %d", i, tc.wantSizes[i], len(batch))
				}
				total += len(batch)
			}

			// every element has to end up in exactly one batch
			if total != len(tc.data) {
				t.Fatalf("expected %d entries across all batches, got %d", len(tc.data), total)
			}
		})
	}
}

// BUG: neither splitter validates its batch size. A negative size panics and a
// size of 0 spins forever, appending empty batches until the process runs out
// of memory — so the zero case deliberately has no test, it would hang the
// suite. Both call sites pass a hard-coded 1000 today, which is why this is not
// reachable in production. Filed as BZE-134.
func TestSplitIntSlicePanicsOnNegativeBatchSize(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected a panic for a negative batch size (BZE-134)")
		}
	}()

	SplitIntSlice([]int{1, 2, 3}, -1)
}

func TestSplitIntervalsSlicePanicsOnNegativeBatchSize(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected a panic for a negative batch size (BZE-134)")
		}
	}()

	SplitIntervalsSlice([]*entity.MarketHistoryInterval{{}}, -1)
}

// an empty input never enters the loop, so the batch size is not consulted
func TestSplitSlicesToleratesZeroBatchSizeOnEmptyInput(t *testing.T) {
	if got := SplitIntSlice(nil, 0); got != nil {
		t.Fatalf("expected no batches, got %v", got)
	}

	if got := SplitIntervalsSlice(nil, 0); got != nil {
		t.Fatalf("expected no batches, got %v", got)
	}
}
