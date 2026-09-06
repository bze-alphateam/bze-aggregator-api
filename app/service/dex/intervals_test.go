package dex

import (
	"errors"
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto/query"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
)

type fakeIntervalStore struct {
	intervals    query.IntervalsMap
	intervalsErr error
	tv           query.TradingIntervalsMap
	tvErr        error
	lastParams   *query.IntervalsParams
}

func (f *fakeIntervalStore) GetIntervalsBy(params *query.IntervalsParams) (query.IntervalsMap, error) {
	f.lastParams = params

	return f.intervals, f.intervalsErr
}

func (f *fakeIntervalStore) GetTradingViewIntervalsBy(params *query.IntervalsParams) (query.TradingIntervalsMap, error) {
	f.lastParams = params

	return f.tv, f.tvErr
}

func newTestIntervalsService(t *testing.T, iRepo *fakeIntervalStore, mRepo *fakeMarketRepo) *Intervals {
	t.Helper()
	svc, err := NewIntervals(iRepo, newTestLogger(), mRepo)
	if err != nil {
		t.Fatalf("NewIntervals: unexpected error: %v", err)
	}

	return svc
}

func TestNewIntervalsRejectsNilDependencies(t *testing.T) {
	logger := newTestLogger()
	iRepo := &fakeIntervalStore{}
	mRepo := &fakeMarketRepo{}

	if svc, err := NewIntervals(iRepo, logger, mRepo); err != nil || svc == nil {
		t.Fatalf("valid dependencies: got svc=%v err=%v", svc, err)
	}

	cases := map[string]func() (*Intervals, error){
		"nil intervalStore": func() (*Intervals, error) { return NewIntervals(nil, logger, mRepo) },
		"nil logger":        func() (*Intervals, error) { return NewIntervals(iRepo, nil, mRepo) },
		"nil marketRepo":    func() (*Intervals, error) { return NewIntervals(iRepo, logger, nil) },
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			if svc, err := build(); err == nil || svc != nil {
				t.Fatalf("expected an error, got svc=%v err=%v", svc, err)
			}
		})
	}
}

// --- getIntervalDuration -----------------------------------------------------

// TestGetIntervalDuration covers every supported `minutes` value plus an
// unsupported one; the helper is a plain minutes->Duration multiplication and
// does not validate the length, so an unsupported value is echoed back too.
func TestGetIntervalDuration(t *testing.T) {
	svc := newTestIntervalsService(t, &fakeIntervalStore{}, &fakeMarketRepo{})

	for _, length := range []int{5, 15, 60, 240, 1440, 7 /* unsupported */} {
		want := time.Duration(length) * time.Minute
		if got := svc.getIntervalDuration(length); got != want {
			t.Fatalf("getIntervalDuration(%d): got %v, want %v", length, got, want)
		}
	}
}

// --- getQueryParams ----------------------------------------------------------

func TestGetQueryParamsLimitedWindow(t *testing.T) {
	// createdAt far in the past so the requested window does not hit the floor.
	created := time.Now().Add(-10000 * time.Minute)
	market := &entity.Market{MarketID: "ubze/uvdl", CreatedAt: created}
	svc := newTestIntervalsService(t, &fakeIntervalStore{}, &fakeMarketRepo{})

	const (
		length = 5
		limit  = 5
	)
	qp := svc.getQueryParams(market, length, limit)

	if qp.MarketId != market.MarketID || qp.Length != length || qp.Limit != limit {
		t.Fatalf("passthrough fields wrong: %+v", qp)
	}
	if !qp.StartAt.After(created) {
		t.Fatalf("start_at should be after created_at, got %v", qp.StartAt)
	}
	// start_at ~= now - limit*duration.
	want := time.Now().Add(-time.Duration(limit) * time.Duration(length) * time.Minute)
	if diff := qp.StartAt.Sub(want); diff > 5*time.Second || diff < -5*time.Second {
		t.Fatalf("start_at: got %v, want ~%v (diff %v)", qp.StartAt, want, diff)
	}
}

// TestGetQueryParamsHardFloorAtCreatedAt: the intervals API must never look
// further back than the market's created-at, even when a large limit asks for it.
func TestGetQueryParamsHardFloorAtCreatedAt(t *testing.T) {
	created := time.Now() // recent -> the requested window reaches before it
	market := &entity.Market{MarketID: "ubze/uvdl", CreatedAt: created}
	svc := newTestIntervalsService(t, &fakeIntervalStore{}, &fakeMarketRepo{})

	qp := svc.getQueryParams(market, 5, 100) // now - 500min is well before createdAt
	if !qp.StartAt.Equal(created) {
		t.Fatalf("start_at should be clamped to created_at %v, got %v", created, qp.StartAt)
	}
}

// TestGetQueryParamsNoLimitStartsAtCreatedAt: limit <= 0 anchors the window at
// created-at.
func TestGetQueryParamsNoLimitStartsAtCreatedAt(t *testing.T) {
	created := time.Now().Add(-100 * time.Minute)
	market := &entity.Market{MarketID: "ubze/uvdl", CreatedAt: created}
	svc := newTestIntervalsService(t, &fakeIntervalStore{}, &fakeMarketRepo{})

	qp := svc.getQueryParams(market, 5, 0)
	if !qp.StartAt.Equal(created) {
		t.Fatalf("start_at should equal created_at %v, got %v", created, qp.StartAt)
	}
}

// --- sorting -----------------------------------------------------------------

func TestSortIntervalsAscendingByStartAt(t *testing.T) {
	svc := newTestIntervalsService(t, &fakeIntervalStore{}, &fakeMarketRepo{})

	in := []entity.MarketHistoryInterval{
		{StartAt: time.Unix(300, 0)},
		{StartAt: time.Unix(100, 0)},
		{StartAt: time.Unix(200, 0)},
	}
	svc.sortIntervals(in)

	for i := 1; i < len(in); i++ {
		if in[i-1].StartAt.After(in[i].StartAt) {
			t.Fatalf("not sorted ascending: %v", in)
		}
	}
}

func TestSortTradingViewIntervalsAscendingByStartAt(t *testing.T) {
	svc := newTestIntervalsService(t, &fakeIntervalStore{}, &fakeMarketRepo{})

	in := []entity.TradingViewInterval{
		{StartAt: time.Unix(300, 0)},
		{StartAt: time.Unix(100, 0)},
		{StartAt: time.Unix(200, 0)},
	}
	svc.sortTradingViewIntervals(in)

	for i := 1; i < len(in); i++ {
		if in[i-1].StartAt.After(in[i].StartAt) {
			t.Fatalf("not sorted ascending: %v", in)
		}
	}
}

// --- GetIntervals ------------------------------------------------------------

// TestGetIntervalsReturnsStoredCandlesSorted uses the exact-match branch
// (len(entries) == limit) so the result is the stored candles, sorted ascending,
// with no zero-fill (which is time-dependent).
func TestGetIntervalsReturnsStoredCandlesSorted(t *testing.T) {
	market := &entity.Market{MarketID: "ubze/uvdl", CreatedAt: time.Now().Add(-10000 * time.Minute)}
	mRepo := &fakeMarketRepo{market: market}
	iRepo := &fakeIntervalStore{intervals: query.IntervalsMap{
		200: {MarketID: market.MarketID, StartAt: time.Unix(200, 0), OpenPrice: "2"},
		100: {MarketID: market.MarketID, StartAt: time.Unix(100, 0), OpenPrice: "1"},
	}}

	svc := newTestIntervalsService(t, iRepo, mRepo)

	got, err := svc.GetIntervals(market.MarketID, 5, 2)
	if err != nil {
		t.Fatalf("GetIntervals: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 candles, got %d", len(got))
	}
	if got[0].StartAt.After(got[1].StartAt) {
		t.Fatalf("candles not sorted ascending: %v", got)
	}
	assertString(t, "candles[0].open", got[0].OpenPrice, "1")
	assertString(t, "candles[1].open", got[1].OpenPrice, "2")
}

// TestGetIntervalsNoCandlesReturnsEmpty: with limit 0 and no stored candles the
// exact-match branch returns an empty result.
func TestGetIntervalsNoCandlesReturnsEmpty(t *testing.T) {
	market := &entity.Market{MarketID: "ubze/uvdl", CreatedAt: time.Now()}
	mRepo := &fakeMarketRepo{market: market}
	iRepo := &fakeIntervalStore{intervals: query.IntervalsMap{}}

	svc := newTestIntervalsService(t, iRepo, mRepo)

	got, err := svc.GetIntervals(market.MarketID, 5, 0)
	if err != nil {
		t.Fatalf("GetIntervals: unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no candles, got %d", len(got))
	}
}

// TestGetIntervalsFillsMissingCandlesWithZeros: when the store returns fewer
// candles than requested, the gap between now and the market's created-at is
// back-filled with zero-valued candles. The exact count depends on wall-clock
// alignment, so this asserts the robust invariants: at least one candle, every
// filled candle zero-valued for this market, sorted ascending.
func TestGetIntervalsFillsMissingCandlesWithZeros(t *testing.T) {
	const length = 5
	market := &entity.Market{MarketID: "ubze/uvdl", CreatedAt: time.Now().Add(-12 * time.Minute)}
	mRepo := &fakeMarketRepo{market: market}
	iRepo := &fakeIntervalStore{intervals: query.IntervalsMap{}} // empty -> everything is filled

	svc := newTestIntervalsService(t, iRepo, mRepo)

	got, err := svc.GetIntervals(market.MarketID, length, 10)
	if err != nil {
		t.Fatalf("GetIntervals: unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("expected at least one filled candle")
	}
	for i, c := range got {
		if c.MarketID != market.MarketID {
			t.Fatalf("candle %d has wrong market id %q", i, c.MarketID)
		}
		if c.OpenPrice != "0" || c.ClosePrice != "0" || c.BaseVolume != "0" || c.QuoteVolume != "0" {
			t.Fatalf("filled candle %d is not zero-valued: %+v", i, c)
		}
		if i > 0 && got[i-1].StartAt.After(c.StartAt) {
			t.Fatalf("filled candles not sorted ascending: %v", got)
		}
	}
}

func TestGetTradingViewIntervalsFillsMissingCandlesWithZeros(t *testing.T) {
	const length = 5
	market := &entity.Market{MarketID: "ubze/uvdl", CreatedAt: time.Now().Add(-12 * time.Minute)}
	mRepo := &fakeMarketRepo{market: market}
	iRepo := &fakeIntervalStore{tv: query.TradingIntervalsMap{}}

	svc := newTestIntervalsService(t, iRepo, mRepo)

	got, err := svc.GetTradingViewIntervals(market.MarketID, length, 10)
	if err != nil {
		t.Fatalf("GetTradingViewIntervals: unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("expected at least one filled candle")
	}
	for i, c := range got {
		if c.OpenPrice != 0 || c.ClosePrice != 0 || c.BaseVolume != 0 {
			t.Fatalf("filled candle %d is not zero-valued: %+v", i, c)
		}
		if i > 0 && got[i-1].StartAt.After(c.StartAt) {
			t.Fatalf("filled candles not sorted ascending: %v", got)
		}
	}
}

func TestGetIntervalsMarketNotFound(t *testing.T) {
	mRepo := &fakeMarketRepo{market: nil}
	svc := newTestIntervalsService(t, &fakeIntervalStore{}, mRepo)

	if _, err := svc.GetIntervals("nope", 5, 10); err == nil {
		t.Fatalf("expected an error for an unknown market")
	}
}

func TestGetIntervalsMarketRepoErrorPropagates(t *testing.T) {
	mRepo := &fakeMarketRepo{err: errors.New("market repo down")}
	svc := newTestIntervalsService(t, &fakeIntervalStore{}, mRepo)

	if _, err := svc.GetIntervals("ubze/uvdl", 5, 10); err == nil {
		t.Fatalf("expected the market-repo error to propagate")
	}
}

func TestGetIntervalsStoreErrorReturnsGenericError(t *testing.T) {
	market := &entity.Market{MarketID: "ubze/uvdl", CreatedAt: time.Now().Add(-10000 * time.Minute)}
	mRepo := &fakeMarketRepo{market: market}
	iRepo := &fakeIntervalStore{intervalsErr: errors.New("store down")}

	svc := newTestIntervalsService(t, iRepo, mRepo)

	if _, err := svc.GetIntervals(market.MarketID, 5, 10); err == nil {
		t.Fatalf("expected an error when the interval store fails")
	}
}

// --- GetTradingViewIntervals -------------------------------------------------

func TestGetTradingViewIntervalsReturnsStoredCandlesSorted(t *testing.T) {
	market := &entity.Market{MarketID: "ubze/uvdl", CreatedAt: time.Now().Add(-10000 * time.Minute)}
	mRepo := &fakeMarketRepo{market: market}
	iRepo := &fakeIntervalStore{tv: query.TradingIntervalsMap{
		200: {StartAt: time.Unix(200, 0), OpenPrice: 2},
		100: {StartAt: time.Unix(100, 0), OpenPrice: 1},
	}}

	svc := newTestIntervalsService(t, iRepo, mRepo)

	got, err := svc.GetTradingViewIntervals(market.MarketID, 5, 2)
	if err != nil {
		t.Fatalf("GetTradingViewIntervals: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 candles, got %d", len(got))
	}
	if got[0].StartAt.After(got[1].StartAt) {
		t.Fatalf("candles not sorted ascending: %v", got)
	}
	if got[0].OpenPrice != 1 || got[1].OpenPrice != 2 {
		t.Fatalf("open prices out of order: %v", got)
	}
}

func TestGetTradingViewIntervalsMarketNotFound(t *testing.T) {
	mRepo := &fakeMarketRepo{market: nil}
	svc := newTestIntervalsService(t, &fakeIntervalStore{}, mRepo)

	if _, err := svc.GetTradingViewIntervals("nope", 5, 10); err == nil {
		t.Fatalf("expected an error for an unknown market")
	}
}

func TestGetTradingViewIntervalsStoreErrorReturnsGenericError(t *testing.T) {
	market := &entity.Market{MarketID: "ubze/uvdl", CreatedAt: time.Now().Add(-10000 * time.Minute)}
	mRepo := &fakeMarketRepo{market: market}
	iRepo := &fakeIntervalStore{tvErr: errors.New("store down")}

	svc := newTestIntervalsService(t, iRepo, mRepo)

	if _, err := svc.GetTradingViewIntervals(market.MarketID, 5, 10); err == nil {
		t.Fatalf("expected an error when the interval store fails")
	}
}
