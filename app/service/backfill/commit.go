package backfill

import (
	"fmt"
	"sort"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/interval"
)

// MarketPlan is what the commit will do to one pool.
type MarketPlan struct {
	MarketID string

	// LiveFloor is the oldest swap the aggregator already holds for this pool.
	// Everything staged at or after it was ingested by the listener while the
	// scan was running and is dropped. Nil when the pool has no history yet.
	LiveFloor *time.Time

	// SharedDay is the candle window LiveFloor falls in - the only one the
	// back-fill and the listener have in common. Its rows are committed without
	// candles and with i_added_to_interval = 0, so the listener rebuilds them
	// from the complete set. Nil when there is no LiveFloor.
	SharedDay *time.Time

	Eligible entity.StagedStats
	Skipped  entity.StagedStats
}

// CommitPlan is the full pre-commit report: what gets inserted, what gets
// dropped as already-ingested, and over which days.
type CommitPlan struct {
	Checkpoint *entity.BackfillCheckpoint
	Counts     entity.StagedCounts
	Markets    []MarketPlan
	Eligible   int64
	Skipped    int64
	FirstDay   time.Time
	LastDay    time.Time
}

// MarketDating records a pool's new i_created_at.
type MarketDating struct {
	MarketID  string
	CreatedAt time.Time
}

type CommitResult struct {
	Days            int
	Rows            int64
	DeferredRows    int64
	Intervals       int64
	Markets         []MarketDating
	CheckpointTable string
	StagedTable     string
}

// CommitPlan works out what a commit would do without changing anything.
func (s *Service) CommitPlan() (*CommitPlan, error) {
	cp, err := s.requireCheckpoint()
	if err != nil {
		return nil, err
	}

	if !cp.ScanComplete {
		return nil, fmt.Errorf(
			"the scan is not complete (at height %d, floor %d) - finish it with `swap-backfill run`. Committing a partial scan and extending it afterwards is not supported",
			cp.NextHeight, cp.FloorHeight,
		)
	}

	counts, err := s.repo.GetStagedCounts()
	if err != nil {
		return nil, err
	}

	floors, err := s.liveFloors()
	if err != nil {
		return nil, err
	}

	marketIds, err := s.repo.GetStagedMarketIds(true)
	if err != nil {
		return nil, err
	}

	plan := &CommitPlan{Checkpoint: cp, Counts: counts}
	for _, marketId := range marketIds {
		marketPlan := MarketPlan{MarketID: marketId}

		var floorPtr *time.Time
		if floor, ok := floors[marketId]; ok {
			sharedDay := dayStart(floor)
			marketPlan.LiveFloor = &floor
			marketPlan.SharedDay = &sharedDay
			floorPtr = &floor
		}

		// eligible = staged rows strictly older than what we already hold
		marketPlan.Eligible, err = s.repo.GetStagedStats(marketId, nil, floorPtr)
		if err != nil {
			return nil, err
		}

		if floorPtr != nil {
			marketPlan.Skipped, err = s.repo.GetStagedStats(marketId, floorPtr, nil)
			if err != nil {
				return nil, err
			}
		}

		plan.Eligible += marketPlan.Eligible.Count
		plan.Skipped += marketPlan.Skipped.Count

		if marketPlan.Eligible.MinExecutedAt.Valid {
			first := dayStart(marketPlan.Eligible.MinExecutedAt.Time)
			if plan.FirstDay.IsZero() || first.Before(plan.FirstDay) {
				plan.FirstDay = first
			}
		}

		if marketPlan.Eligible.MaxExecutedAt.Valid {
			last := dayStart(marketPlan.Eligible.MaxExecutedAt.Time)
			if plan.LastDay.IsZero() || last.After(plan.LastDay) {
				plan.LastDay = last
			}
		}

		plan.Markets = append(plan.Markets, marketPlan)
	}

	return plan, nil
}

// Commit moves the staged swaps into market_history and writes their candles.
//
// It works one day at a time - a day is the biggest candle bucket, so every
// candle length is window-complete inside the chunk - and each day is a single
// transaction that inserts the rows, upserts the candles and flips the staged
// rows' committed flag together. That atomicity is the whole duplicate story:
// a killed commit resumes with the rows that are still flagged uncommitted and
// inserts nothing twice.
func (s *Service) Commit(plan *CommitPlan) (*CommitResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("no commit plan")
	}

	// the checkpoint is re-read: the plan may have been printed and confirmed
	// minutes ago
	cp, err := s.requireCheckpoint()
	if err != nil {
		return nil, err
	}

	if !cp.ScanComplete {
		return nil, fmt.Errorf("the scan is not complete - finish it with `swap-backfill run`")
	}

	// the guard is re-read instead of taken from the plan: the plan may have
	// been printed and confirmed minutes ago, and the listener never stops. A
	// floor can only stay put or move earlier, which shrinks what we insert -
	// so the plan's day range still covers everything.
	floors, err := s.liveFloors()
	if err != nil {
		return nil, err
	}

	sharedDays := make(map[string]time.Time)
	for marketId, floor := range floors {
		sharedDays[marketId] = dayStart(floor)
	}

	result := &CommitResult{}

	if plan.Eligible > 0 {
		for day := plan.FirstDay; !day.After(plan.LastDay); day = day.Add(dayDuration) {
			committed, deferred, intervals, err := s.commitDay(day, floors, sharedDays)
			if err != nil {
				return nil, fmt.Errorf("could not commit %s: %w", day.Format(time.DateOnly), err)
			}

			if committed == 0 {
				continue
			}

			result.Days++
			result.Rows += committed
			result.DeferredRows += deferred
			result.Intervals += intervals
			s.logger.Infof("committed %s: %d rows, %d candles (%d rows left for the listener)", day.Format(time.DateOnly), committed, intervals, deferred)
		}
	} else {
		s.logger.Info("nothing left to commit - all staged rows are already committed or were skipped by the overlap guard")
	}

	result.Markets, err = s.dateMarkets()
	if err != nil {
		return nil, err
	}

	result.CheckpointTable, result.StagedTable, err = s.repo.RenameTables(time.Now().UTC().Format("20060102_150405"))
	if err != nil {
		return nil, fmt.Errorf("everything was committed but the staging tables could not be renamed: %w", err)
	}

	return result, nil
}

// commitDay commits one day bucket in a single transaction.
func (s *Service) commitDay(day time.Time, floors, sharedDays map[string]time.Time) (committed, deferred, intervalCount int64, err error) {
	staged, err := s.repo.GetUncommittedBetween(day, day.Add(dayDuration))
	if err != nil {
		return 0, 0, 0, err
	}

	grouped := make(map[string][]entity.StagedSwap)
	for _, row := range staged {
		// the overlap guard: anything at or after the pool's oldest live trade
		// was already ingested from Postgres by the listener
		if floor, ok := floors[row.MarketID]; ok && !row.ExecutedAt.Before(floor) {
			continue
		}

		grouped[row.MarketID] = append(grouped[row.MarketID], row)
	}

	marketIds := make([]string, 0, len(grouped))
	for marketId := range grouped {
		marketIds = append(marketIds, marketId)
	}
	sort.Strings(marketIds)

	var rows []*entity.MarketHistory
	var intervals []*entity.MarketHistoryInterval
	var ids []int
	for _, marketId := range marketIds {
		// the day the live data starts in is the one window we share with the
		// listener: leave its candles to it, it sees both halves
		sharedDay, hasShared := sharedDays[marketId]
		ownCandles := !hasShared || !sharedDay.Equal(day)

		iMap := interval.NewIntervalsMap(marketId)
		for _, row := range grouped[marketId] {
			hist := row.ToMarketHistory(ownCandles)
			rows = append(rows, hist)
			ids = append(ids, row.ID)

			if ownCandles {
				iMap.AddOrder(hist)
			} else {
				deferred++
			}
		}

		if ownCandles {
			intervals = append(intervals, converter.IntervalMapToEntities(iMap)...)
		}
	}

	if len(rows) == 0 {
		return 0, 0, 0, nil
	}

	if err = s.repo.CommitChunk(rows, intervals, ids); err != nil {
		return 0, 0, 0, err
	}

	return int64(len(rows)), deferred, int64(len(intervals)), nil
}

// dateMarkets moves every back-filled pool's i_created_at back to cover the
// recovered candles. Without it the data sits in MySQL and stays invisible:
// dex.Intervals uses market.CreatedAt as a hard floor and LP markets are
// created with time.Now().
//
// The value is the oldest swap floored to the biggest bucket (minus a minute of
// margin), not the swap's own timestamp: the API filters on the bucket start,
// which is always at or before the trade it contains.
func (s *Service) dateMarkets() ([]MarketDating, error) {
	// every market in the staging table, not just what this run committed: a
	// commit that died after its last day still has to be able to finish here
	staged, err := s.repo.GetStagedMarketIds(false)
	if err != nil {
		return nil, err
	}

	touched := make(map[string]bool, len(staged))
	for _, marketId := range staged {
		touched[marketId] = true
	}

	pools, err := s.repo.GetPoolsOldestSwap(false)
	if err != nil {
		return nil, err
	}

	var dated []MarketDating
	for _, pool := range pools {
		if !touched[pool.MarketID] || !pool.OldestSwap.Valid {
			continue
		}

		createdAt := dayStart(pool.OldestSwap.Time).Add(-time.Minute)
		if err = s.markets.UpdateCreatedAt(pool.MarketID, createdAt); err != nil {
			return nil, fmt.Errorf("could not update the created date of %s: %w", pool.MarketID, err)
		}

		s.logger.Infof("dated market %s back to %s", pool.MarketID, createdAt.Format(time.RFC3339))
		dated = append(dated, MarketDating{MarketID: pool.MarketID, CreatedAt: createdAt})
	}

	return dated, nil
}

// liveFloors is the oldest swap the listener already ingested per pool - the
// overlap guard. market_history is the right yardstick here: it is our own
// durable data, while the Postgres block index is pruned by the cleanup cron
// and would understate what has been ingested.
//
// Rows this back-fill committed are left out. They are older than the floor by
// construction, so counting them would move the guard back onto our own work:
// a commit resumed after an interrupted day would then treat every day it had
// not reached yet as already-held and silently drop it.
func (s *Service) liveFloors() (map[string]time.Time, error) {
	pools, err := s.repo.GetPoolsOldestSwap(true)
	if err != nil {
		return nil, err
	}

	floors := make(map[string]time.Time)
	for _, pool := range pools {
		if pool.OldestSwap.Valid {
			floors[pool.MarketID] = pool.OldestSwap.Time
		}
	}

	return floors, nil
}
