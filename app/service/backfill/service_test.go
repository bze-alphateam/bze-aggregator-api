package backfill

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	abci "github.com/cometbft/cometbft/abci/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/sirupsen/logrus"
)

type fakeRepo struct {
	tables bool
	cp     *entity.BackfillCheckpoint
	staged []entity.StagedSwap
	// liveFloors is what market_history already holds per pool before the
	// back-fill runs
	liveFloors map[string]time.Time

	inserted  []*entity.MarketHistory
	intervals []*entity.MarketHistoryInterval

	commitCalls  int
	failCommitAt int
	renamed      string

	scanRows      []*entity.StagedSwap
	savedNext     int64
	savedComplete bool
}

func (f *fakeRepo) TablesExist() (bool, error) { return f.tables, nil }

func (f *fakeRepo) CreateTables() error {
	f.tables = true

	return nil
}

func (f *fakeRepo) DropTables() error {
	f.tables = false

	return nil
}

func (f *fakeRepo) RenameTables(suffix string) (string, string, error) {
	f.renamed = suffix

	return "swap_backfill_checkpoint_" + suffix, "swap_backfill_history_" + suffix, nil
}

func (f *fakeRepo) SaveCheckpoint(cp *entity.BackfillCheckpoint) error {
	f.cp = cp

	return nil
}

func (f *fakeRepo) GetCheckpoint() (*entity.BackfillCheckpoint, error) { return f.cp, nil }

func (f *fakeRepo) GetStagedCounts() (entity.StagedCounts, error) {
	counts := entity.StagedCounts{Total: int64(len(f.staged))}
	for _, row := range f.staged {
		if row.Committed {
			counts.Committed++
		}
	}

	return counts, nil
}

func (f *fakeRepo) SaveScanChunk(rows []*entity.StagedSwap, nextHeight int64, scanComplete bool) error {
	f.scanRows = append(f.scanRows, rows...)
	f.savedNext = nextHeight
	f.savedComplete = scanComplete
	if f.cp != nil {
		f.cp.NextHeight = nextHeight
		f.cp.ScanComplete = scanComplete
	}

	return nil
}

func (f *fakeRepo) GetStagedMarketIds(uncommittedOnly bool) ([]string, error) {
	seen := make(map[string]bool)
	for _, row := range f.staged {
		if !uncommittedOnly || !row.Committed {
			seen[row.MarketID] = true
		}
	}

	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	return ids, nil
}

func (f *fakeRepo) GetStagedStats(marketId string, from, to *time.Time) (entity.StagedStats, error) {
	stats := entity.StagedStats{}
	for _, row := range f.staged {
		if row.Committed || row.MarketID != marketId {
			continue
		}

		if from != nil && row.ExecutedAt.Before(*from) {
			continue
		}

		if to != nil && !row.ExecutedAt.Before(*to) {
			continue
		}

		stats.Count++
		if !stats.MinExecutedAt.Valid || row.ExecutedAt.Before(stats.MinExecutedAt.Time) {
			stats.MinExecutedAt = sql.NullTime{Time: row.ExecutedAt, Valid: true}
		}

		if !stats.MaxExecutedAt.Valid || row.ExecutedAt.After(stats.MaxExecutedAt.Time) {
			stats.MaxExecutedAt = sql.NullTime{Time: row.ExecutedAt, Valid: true}
		}

		if !stats.MinHeight.Valid || row.Height < stats.MinHeight.Int64 {
			stats.MinHeight = sql.NullInt64{Int64: row.Height, Valid: true}
		}

		if !stats.MaxHeight.Valid || row.Height > stats.MaxHeight.Int64 {
			stats.MaxHeight = sql.NullInt64{Int64: row.Height, Valid: true}
		}
	}

	return stats, nil
}

func (f *fakeRepo) GetUncommittedBetween(from, to time.Time) ([]entity.StagedSwap, error) {
	var rows []entity.StagedSwap
	for _, row := range f.staged {
		if row.Committed || row.ExecutedAt.Before(from) || !row.ExecutedAt.Before(to) {
			continue
		}

		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Height != rows[j].Height {
			return rows[i].Height < rows[j].Height
		}

		return rows[i].EventIndex < rows[j].EventIndex
	})

	return rows, nil
}

// CommitChunk mimics the real transaction: either the rows land and the flags
// flip, or nothing happens at all.
func (f *fakeRepo) CommitChunk(rows []*entity.MarketHistory, intervals []*entity.MarketHistoryInterval, stagedIds []int) error {
	if len(rows) == 0 {
		return nil
	}

	f.commitCalls++
	if f.commitCalls == f.failCommitAt {
		return fmt.Errorf("boom")
	}

	f.inserted = append(f.inserted, rows...)
	f.intervals = append(f.intervals, intervals...)
	for _, id := range stagedIds {
		for i := range f.staged {
			if f.staged[i].ID == id {
				f.staged[i].Committed = true
			}
		}
	}

	return nil
}

// GetPoolsOldestSwap answers like the live query would: the oldest of what the
// listener already had and what the back-fill has inserted so far. With
// excludeBackfilled the inserted rows drop out, leaving the listener's own
// floor - which is what the overlap guard asks for.
func (f *fakeRepo) GetPoolsOldestSwap(excludeBackfilled bool) ([]entity.PoolOldestSwap, error) {
	oldest := make(map[string]time.Time)
	for marketId, floor := range f.liveFloors {
		oldest[marketId] = floor
	}

	if !excludeBackfilled {
		for _, row := range f.inserted {
			current, ok := oldest[row.MarketID]
			if !ok || row.ExecutedAt.Before(current) {
				oldest[row.MarketID] = row.ExecutedAt
			}
		}
	}

	var ids []string
	for id := range oldest {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var pools []entity.PoolOldestSwap
	for _, id := range ids {
		pools = append(pools, entity.PoolOldestSwap{
			MarketID:   id,
			OldestSwap: sql.NullTime{Time: oldest[id], Valid: true},
		})
	}

	return pools, nil
}

type fakeMarkets struct {
	dated map[string]time.Time
}

func (f *fakeMarkets) UpdateCreatedAt(marketId string, createdAt time.Time) error {
	if f.dated == nil {
		f.dated = make(map[string]time.Time)
	}
	f.dated[marketId] = createdAt

	return nil
}

type fakeChain struct {
	height  int64
	swapsAt map[int64]int

	mx        sync.Mutex
	requested []int64
}

func (f *fakeChain) GetStatus() (*coretypes.ResultStatus, error) {
	return &coretypes.ResultStatus{SyncInfo: coretypes.SyncInfo{LatestBlockHeight: f.height}}, nil
}

func (f *fakeChain) GetBlockTime(height int64) (time.Time, error) {
	return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(height) * time.Second), nil
}

func (f *fakeChain) GetBlockResults(ctx context.Context, height int64) (*coretypes.ResultBlockResults, error) {
	f.mx.Lock()
	f.requested = append(f.requested, height)
	f.mx.Unlock()

	res := &coretypes.ResultBlockResults{Height: height}
	for i := 0; i < f.swapsAt[height]; i++ {
		res.TxsResults = append(res.TxsResults, &abci.ExecTxResult{Code: 0, Events: []abci.Event{swapEvent("aaa_bbb")}})
	}

	return res, nil
}

func newTestService(repo *fakeRepo, markets *fakeMarkets, chain *fakeChain) *Service {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	svc, err := NewService(logger, repo, markets, chain, fakeAssets{})
	if err != nil {
		panic(err)
	}

	return svc
}

func TestChunkRangeWalksDescendingAndStopsAtTheFloor(t *testing.T) {
	cp := &entity.BackfillCheckpoint{InitHeight: 20240000, FloorHeight: FloorHeight, NextHeight: 20240000}

	high, low, done := chunkRange(cp, 1000)
	if done || high != 20240000 || low != 20239001 {
		t.Fatalf("unexpected first chunk: %d..%d (done: %t)", low, high, done)
	}

	// resume: the checkpoint is all that decides where the scan continues
	cp.NextHeight = low - 1
	high, low, done = chunkRange(cp, 1000)
	if done || high != 20239000 || low != 20238001 {
		t.Fatalf("unexpected resumed chunk: %d..%d (done: %t)", low, high, done)
	}

	// the last chunk is clamped to the floor, never below it
	cp.NextHeight = FloorHeight + 10
	high, low, done = chunkRange(cp, 1000)
	if done || high != FloorHeight+10 || low != FloorHeight {
		t.Fatalf("expected the chunk to stop at the floor, got %d..%d (done: %t)", low, high, done)
	}

	cp.NextHeight = FloorHeight - 1
	if _, _, done = chunkRange(cp, 1000); !done {
		t.Fatal("expected the scan to be done below the floor")
	}

	// a checkpoint above the initialized range cannot widen it
	cp.NextHeight = cp.InitHeight + 5000
	high, _, _ = chunkRange(cp, 1000)
	if high != cp.InitHeight {
		t.Fatalf("expected the scan to stay inside the initialized range, got %d", high)
	}
}

func TestInitRefusesHeightsOutsideTheAllowedRange(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(repo, &fakeMarkets{}, &fakeChain{height: 20500000})

	if err := svc.Init(FloorHeight - 1); err == nil {
		t.Fatal("expected an error below the liquidity pools upgrade height")
	}

	if err := svc.Init(20500001); err == nil {
		t.Fatal("expected an error above the current chain height")
	}

	if err := svc.Init(20400000); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if repo.cp == nil || repo.cp.NextHeight != 20400000 || repo.cp.FloorHeight != FloorHeight {
		t.Fatalf("unexpected checkpoint: %+v", repo.cp)
	}

	if err := svc.Init(20400000); err == nil {
		t.Fatal("expected an error when a back-fill is already initialized")
	}
}

func TestInitStatusReportsTheBoundary(t *testing.T) {
	oldest := time.Date(2026, 9, 2, 11, 4, 5, 0, time.UTC)
	repo := &fakeRepo{liveFloors: map[string]time.Time{
		"aaa_bbb": oldest.Add(time.Hour),
		"ccc_ddd": oldest,
	}}
	svc := newTestService(repo, &fakeMarkets{}, &fakeChain{height: 20500000})

	status, err := svc.InitStatus()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if status.Initialized {
		t.Fatal("expected an uninitialized back-fill")
	}

	if len(status.Pools) != 2 {
		t.Fatalf("expected both pools to be reported, got %d", len(status.Pools))
	}

	if status.Boundary == nil || !status.Boundary.Equal(oldest) {
		t.Fatalf("expected the lowest timestamp across pools, got %v", status.Boundary)
	}

	if status.CurrentHeight != 20500000 {
		t.Fatalf("unexpected chain height: %d", status.CurrentHeight)
	}
}

func TestCleanupRefusesAfterSomethingWasCommitted(t *testing.T) {
	repo := &fakeRepo{
		tables: true,
		cp:     &entity.BackfillCheckpoint{InitHeight: 20240000, FloorHeight: FloorHeight, NextHeight: FloorHeight - 1, ScanComplete: true},
		staged: []entity.StagedSwap{{ID: 1, MarketID: "aaa_bbb", Committed: true}, {ID: 2, MarketID: "aaa_bbb"}},
	}
	svc := newTestService(repo, &fakeMarkets{}, &fakeChain{})

	if err := svc.Cleanup(false); err == nil {
		t.Fatal("expected cleanup to refuse dropping the only record of what was committed")
	}

	if !repo.tables {
		t.Fatal("expected the tables to still exist")
	}

	if err := svc.Cleanup(true); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if repo.tables {
		t.Fatal("expected the tables to be dropped when forced")
	}
}

func stagedSwap(id int, marketId string, height int64, executedAt time.Time) entity.StagedSwap {
	return entity.StagedSwap{
		ID:          id,
		MarketID:    marketId,
		OrderType:   "buy",
		Amount:      "10",
		Price:       "1.5",
		QuoteAmount: "15",
		ExecutedAt:  executedAt,
		Height:      height,
		Taker:       "bze1creator",
	}
}

// a pool that traded before the rebuild, plus a pool the aggregator has never
// seen any live data for
func commitFixture(t *testing.T) (*fakeRepo, *fakeMarkets, *Service) {
	t.Helper()

	liveFloor := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		tables: true,
		cp: &entity.BackfillCheckpoint{
			InitHeight: 20240000, FloorHeight: FloorHeight, NextHeight: FloorHeight - 1, ScanComplete: true,
		},
		liveFloors: map[string]time.Time{"aaa_bbb": liveFloor},
		staged: []entity.StagedSwap{
			stagedSwap(1, "aaa_bbb", 100, time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)),
			stagedSwap(2, "aaa_bbb", 101, time.Date(2026, 8, 1, 9, 3, 0, 0, time.UTC)),
			stagedSwap(3, "aaa_bbb", 200, time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)),
			// already ingested by the listener while the scan was running
			stagedSwap(4, "aaa_bbb", 300, time.Date(2026, 8, 2, 13, 0, 0, 0, time.UTC)),
			stagedSwap(5, "ccc_ddd", 150, time.Date(2026, 8, 1, 11, 0, 0, 0, time.UTC)),
		},
	}

	markets := &fakeMarkets{}

	return repo, markets, newTestService(repo, markets, &fakeChain{})
}

func TestCommitPlanRefusesAnIncompleteScan(t *testing.T) {
	repo, _, svc := commitFixture(t)
	repo.cp.ScanComplete = false
	repo.cp.NextHeight = 20230000

	if _, err := svc.CommitPlan(); err == nil {
		t.Fatal("expected the commit to refuse a partial scan")
	}
}

func TestCommitPlanSkipsWhatTheListenerAlreadyHas(t *testing.T) {
	_, _, svc := commitFixture(t)

	plan, err := svc.CommitPlan()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if plan.Eligible != 4 || plan.Skipped != 1 {
		t.Fatalf("expected 4 eligible and 1 skipped row, got %d/%d", plan.Eligible, plan.Skipped)
	}

	if !plan.FirstDay.Equal(dayStart(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))) {
		t.Fatalf("unexpected first day: %s", plan.FirstDay)
	}

	if !plan.LastDay.Equal(dayStart(time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC))) {
		t.Fatalf("unexpected last day: %s", plan.LastDay)
	}

	for _, market := range plan.Markets {
		switch market.MarketID {
		case "aaa_bbb":
			if market.LiveFloor == nil || market.SharedDay == nil {
				t.Fatal("expected a live floor and a shared day for aaa_bbb")
			}

			if !market.SharedDay.Equal(dayStart(*market.LiveFloor)) {
				t.Fatalf("the shared day must be the day the live data starts in, got %s", market.SharedDay)
			}
		case "ccc_ddd":
			if market.LiveFloor != nil || market.Skipped.Count != 0 {
				t.Fatal("a pool without live data has nothing to guard against")
			}
		default:
			t.Fatalf("unexpected market in the plan: %s", market.MarketID)
		}
	}
}

func TestCommitWritesCandlesExceptForTheDaySharedWithTheListener(t *testing.T) {
	repo, markets, svc := commitFixture(t)

	plan, err := svc.CommitPlan()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	result, err := svc.Commit(plan)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(repo.inserted) != 4 {
		t.Fatalf("expected 4 inserted rows, got %d", len(repo.inserted))
	}

	for _, row := range repo.inserted {
		sharedDay := row.MarketID == "aaa_bbb" && dayStart(row.ExecutedAt).Equal(dayStart(time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)))
		if sharedDay && row.AddedToInterval {
			t.Fatalf("the day shared with the listener must be left unprocessed: %+v", row)
		}

		if !sharedDay && !row.AddedToInterval {
			t.Fatalf("back-filled candles were written, the row must be marked: %+v", row)
		}
	}

	// 5 candle lengths for each of the two markets that own their day
	if len(repo.intervals) != 10 {
		t.Fatalf("expected 10 candles, got %d", len(repo.intervals))
	}

	for _, candle := range repo.intervals {
		if candle.MarketID == "aaa_bbb" && candle.StartAt.After(time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("no candle may be written for the shared day: %+v", candle)
		}
	}

	if result.DeferredRows != 1 {
		t.Fatalf("expected 1 row left for the listener, got %d", result.DeferredRows)
	}

	// the row the listener already had must stay untouched
	for _, row := range repo.staged {
		if row.ID == 4 && row.Committed {
			t.Fatal("an already ingested row was committed again")
		}
	}

	expectedDate := dayStart(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)).Add(-time.Minute)
	for _, marketId := range []string{"aaa_bbb", "ccc_ddd"} {
		dated, ok := markets.dated[marketId]
		if !ok {
			t.Fatalf("market %s was not dated back", marketId)
		}

		if !dated.Equal(expectedDate) {
			t.Fatalf("expected %s to start at %s, got %s", marketId, expectedDate, dated)
		}
	}

	if repo.renamed == "" {
		t.Fatal("expected the staging tables to be archived, not dropped")
	}
}

func TestCommitResumesAfterACrashWithoutDuplicates(t *testing.T) {
	repo, _, svc := commitFixture(t)

	plan, err := svc.CommitPlan()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// the first day's transaction dies: nothing lands, nothing is flagged
	repo.failCommitAt = 1
	if _, err = svc.Commit(plan); err == nil {
		t.Fatal("expected the commit to fail")
	}

	if len(repo.inserted) != 0 {
		t.Fatalf("a failed transaction must not leave rows behind, got %d", len(repo.inserted))
	}

	repo.failCommitAt = 0
	plan, err = svc.CommitPlan()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if _, err = svc.Commit(plan); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(repo.inserted) != 4 {
		t.Fatalf("expected exactly 4 rows after the resume, got %d", len(repo.inserted))
	}

	seen := make(map[string]bool)
	for _, row := range repo.inserted {
		key := fmt.Sprintf("%s|%s", row.MarketID, row.ExecutedAt)
		if seen[key] {
			t.Fatalf("row %s was inserted twice", key)
		}
		seen[key] = true
	}

	// a second commit has nothing left to do
	plan, err = svc.CommitPlan()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if plan.Eligible != 0 {
		t.Fatalf("expected nothing left to commit, got %d rows", plan.Eligible)
	}
}

func TestCommitResumesAfterADayFailedMidWay(t *testing.T) {
	repo, _, svc := commitFixture(t)

	plan, err := svc.CommitPlan()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// the first day lands, the second one dies. The rows of 2026-08-01 are now
	// in market_history and they are older than anything the listener holds: if
	// the guard read them back as live data, 2026-08-02 would look already-held
	// and be dropped for good.
	repo.failCommitAt = 2
	if _, err = svc.Commit(plan); err == nil {
		t.Fatal("expected the second day to fail")
	}

	if len(repo.inserted) != 3 {
		t.Fatalf("expected the first day to be committed, got %d rows", len(repo.inserted))
	}

	if repo.renamed != "" {
		t.Fatal("a failed commit must keep the staging tables")
	}

	repo.failCommitAt = 0
	plan, err = svc.CommitPlan()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if plan.Eligible != 1 {
		t.Fatalf("expected the unfinished day to still be eligible, got %d rows", plan.Eligible)
	}

	if _, err = svc.Commit(plan); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(repo.inserted) != 4 {
		t.Fatalf("expected all 4 rows after the resume, got %d", len(repo.inserted))
	}

	seen := make(map[string]bool)
	for _, row := range repo.inserted {
		key := fmt.Sprintf("%s|%s", row.MarketID, row.ExecutedAt)
		if seen[key] {
			t.Fatalf("row %s was inserted twice", key)
		}
		seen[key] = true
	}

	for _, row := range repo.staged {
		// id 4 is the row the listener already had - it stays uncommitted
		if row.ID != 4 && !row.Committed {
			t.Fatalf("staged row %d was never committed", row.ID)
		}
	}
}

func TestRunScansDescendingDownToTheFloor(t *testing.T) {
	repo := &fakeRepo{
		tables: true,
		cp: &entity.BackfillCheckpoint{
			InitHeight: FloorHeight + 10, FloorHeight: FloorHeight, NextHeight: FloorHeight + 10,
		},
	}
	chain := &fakeChain{height: FloorHeight + 10, swapsAt: map[int64]int{FloorHeight + 5: 2}}
	svc := newTestService(repo, &fakeMarkets{}, chain)

	if err := svc.Run(context.Background(), RunOptions{Workers: 1, ChunkSize: 5}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if !repo.savedComplete || repo.savedNext != FloorHeight-1 {
		t.Fatalf("expected the scan to finish at the floor, got next %d (complete: %t)", repo.savedNext, repo.savedComplete)
	}

	if len(chain.requested) != 11 {
		t.Fatalf("expected every height of the range to be requested once, got %d", len(chain.requested))
	}

	if chain.requested[0] != FloorHeight+10 || chain.requested[len(chain.requested)-1] != FloorHeight {
		t.Fatalf("expected a descending scan, got %d..%d", chain.requested[0], chain.requested[len(chain.requested)-1])
	}

	if len(repo.scanRows) != 2 {
		t.Fatalf("expected the 2 staged swaps of the block, got %d", len(repo.scanRows))
	}

	for _, row := range repo.scanRows {
		if row.Height != FloorHeight+5 {
			t.Fatalf("unexpected provenance: %+v", row)
		}

		blockTime, _ := chain.GetBlockTime(FloorHeight + 5)
		if !row.ExecutedAt.Equal(blockTime) {
			t.Fatalf("expected the block time as executed_at, got %s", row.ExecutedAt)
		}
	}

	// a finished scan is not scanned again
	chain.requested = nil
	if err := svc.Run(context.Background(), RunOptions{Workers: 1, ChunkSize: 5}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(chain.requested) != 0 {
		t.Fatal("expected a complete scan to do nothing")
	}
}
