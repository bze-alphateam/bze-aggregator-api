// Package backfill recovers liquidity pool swap history from an archive node.
//
// LP swaps are not part of chain state: they only ever existed as emitted
// bze.tradebin.SwapEvent events, which the aggregator normally ingests from the
// node's Postgres event index. When that index is lost, the only way back is to
// walk historical blocks and parse the events out of them.
//
// The scan stages everything into two temp tables and commits in one separate,
// reviewable step. That is what keeps it safe next to a running listener: while
// the scan runs (which can take days) the two processes share nothing - no
// rows, no candles, no flags - and at commit time the whole range is present,
// so every candle is computed once from a complete window.
package backfill

import (
	"context"
	"fmt"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/interval"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/sirupsen/logrus"
)

// FloorHeight is the upgrade height that introduced liquidity pools. No swap
// can exist before it, so the scan never goes lower - there is nothing there.
const FloorHeight int64 = 20237800

const dayDuration = 24 * time.Hour

type backfillRepo interface {
	TablesExist() (bool, error)
	CreateTables() error
	DropTables() error
	RenameTables(suffix string) (checkpointName, stagedName string, err error)
	SaveCheckpoint(cp *entity.BackfillCheckpoint) error
	GetCheckpoint() (*entity.BackfillCheckpoint, error)
	GetStagedCounts() (entity.StagedCounts, error)
	SaveScanChunk(rows []*entity.StagedSwap, nextHeight int64, scanComplete bool) error
	GetStagedMarketIds(uncommittedOnly bool) ([]string, error)
	GetStagedStats(marketId string, from, to *time.Time) (entity.StagedStats, error)
	GetUncommittedBetween(from, to time.Time) ([]entity.StagedSwap, error)
	CommitChunk(rows []*entity.MarketHistory, intervals []*entity.MarketHistoryInterval, stagedIds []int) error
	GetPoolsOldestSwap() ([]entity.PoolOldestSwap, error)
}

type marketRepo interface {
	UpdateCreatedAt(marketId string, createdAt time.Time) error
}

type chainProvider interface {
	GetStatus() (*coretypes.ResultStatus, error)
	GetBlockTime(height int64) (time.Time, error)
	GetBlockResults(ctx context.Context, height int64) (*coretypes.ResultBlockResults, error)
}

type assetProvider interface {
	GetAssetDetails(denom string) (*chain_registry.ChainRegistryAsset, error)
}

type Service struct {
	logger  logrus.FieldLogger
	repo    backfillRepo
	markets marketRepo
	chain   chainProvider
	assets  assetProvider
}

func NewService(logger logrus.FieldLogger, repo backfillRepo, markets marketRepo, chain chainProvider, assets assetProvider) (*Service, error) {
	if logger == nil || repo == nil || markets == nil || chain == nil || assets == nil {
		return nil, internal.NewInvalidDependenciesErr("NewBackfillService")
	}

	return &Service{
		logger:  logger.WithField("service", "SwapBackfill"),
		repo:    repo,
		markets: markets,
		chain:   chain,
		assets:  assets,
	}, nil
}

// PoolBoundary is the oldest swap the aggregator still holds for a pool.
// Everything before it is what the back-fill has to recover.
type PoolBoundary struct {
	MarketID   string
	OldestSwap *time.Time
}

// InitStatus is everything init knows before it creates anything.
type InitStatus struct {
	// Initialized is true when a previous init is still in place. Everything
	// below except Checkpoint/Counts is then left empty on purpose: with a
	// checkpoint present init refuses to do anything anyway.
	Initialized bool
	Checkpoint  *entity.BackfillCheckpoint
	Counts      entity.StagedCounts

	// Pools and Boundary answer the only question the operator cannot answer
	// alone: from which point in time is the data missing. Nothing on chain or
	// in either database records it, so the closest we can get is the oldest
	// swap we still hold.
	Pools         []PoolBoundary
	Boundary      *time.Time
	CurrentHeight int64
	FloorHeight   int64
}

// Status is the state of an initialized back-fill.
type Status struct {
	Checkpoint *entity.BackfillCheckpoint
	Counts     entity.StagedCounts
}

// InitStatus reports what init would work with: either the existing checkpoint,
// or the boundary timestamps the operator needs to pick a start height.
func (s *Service) InitStatus() (*InitStatus, error) {
	exists, err := s.repo.TablesExist()
	if err != nil {
		return nil, err
	}

	if exists {
		status, err := s.Status()
		if err != nil {
			return nil, err
		}

		return &InitStatus{
			Initialized: true,
			Checkpoint:  status.Checkpoint,
			Counts:      status.Counts,
			FloorHeight: FloorHeight,
		}, nil
	}

	result := &InitStatus{FloorHeight: FloorHeight}
	pools, err := s.repo.GetPoolsOldestSwap()
	if err != nil {
		return nil, err
	}

	for _, pool := range pools {
		boundary := PoolBoundary{MarketID: pool.MarketID}
		if pool.OldestSwap.Valid {
			oldest := pool.OldestSwap.Time
			boundary.OldestSwap = &oldest
			if result.Boundary == nil || oldest.Before(*result.Boundary) {
				result.Boundary = &oldest
			}
		}

		result.Pools = append(result.Pools, boundary)
	}

	chainStatus, err := s.chain.GetStatus()
	if err != nil {
		return nil, fmt.Errorf("could not read the current chain height: %w", err)
	}
	result.CurrentHeight = chainStatus.SyncInfo.LatestBlockHeight

	return result, nil
}

// Init creates the staging tables and the checkpoint the scan resumes from.
func (s *Service) Init(height int64) error {
	exists, err := s.repo.TablesExist()
	if err != nil {
		return err
	}

	if exists {
		return fmt.Errorf("a back-fill is already initialized - review it or run `swap-backfill cleanup` first")
	}

	chainStatus, err := s.chain.GetStatus()
	if err != nil {
		return fmt.Errorf("could not read the current chain height: %w", err)
	}

	currentHeight := chainStatus.SyncInfo.LatestBlockHeight
	if height < FloorHeight {
		return fmt.Errorf("height %d is below %d, the upgrade that introduced liquidity pools - there are no swaps there", height, FloorHeight)
	}

	if height > currentHeight {
		return fmt.Errorf("height %d is above the current chain height %d", height, currentHeight)
	}

	if err = s.repo.CreateTables(); err != nil {
		return fmt.Errorf("could not create the staging tables: %w", err)
	}

	now := time.Now()
	err = s.repo.SaveCheckpoint(&entity.BackfillCheckpoint{
		InitHeight:  height,
		FloorHeight: FloorHeight,
		NextHeight:  height,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return fmt.Errorf("could not save the checkpoint: %w", err)
	}

	s.logger.Infof("initialized swap back-fill: scanning %d down to %d", height, FloorHeight)

	return nil
}

// Status returns the checkpoint and the staged row counts. The checkpoint may
// be nil when the tables are there but the row is not: that is a broken
// back-fill, and cleanup still has to be able to report and drop it.
func (s *Service) Status() (*Status, error) {
	exists, err := s.repo.TablesExist()
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("no back-fill is initialized - run `swap-backfill init` first")
	}

	cp, err := s.repo.GetCheckpoint()
	if err != nil {
		return nil, err
	}

	counts, err := s.repo.GetStagedCounts()
	if err != nil {
		return nil, err
	}

	return &Status{Checkpoint: cp, Counts: counts}, nil
}

// Cleanup drops the staging tables.
//
// It refuses once anything has been committed: the staging tables are then the
// only record of what was already inserted into market_history, and dropping
// them before re-running init would duplicate everything with nothing left to
// detect it.
func (s *Service) Cleanup(force bool) error {
	status, err := s.Status()
	if err != nil {
		return err
	}

	if status.Counts.Committed > 0 && !force {
		return fmt.Errorf(
			"%d staged rows were already committed to market_history - these tables are the only record of that. Dropping them and re-running the back-fill would duplicate those rows undetectably. Use --force only if you know what you are doing",
			status.Counts.Committed,
		)
	}

	if err = s.repo.DropTables(); err != nil {
		return err
	}

	s.logger.Info("staging tables dropped")

	return nil
}

func (s *Service) requireCheckpoint() (*entity.BackfillCheckpoint, error) {
	exists, err := s.repo.TablesExist()
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("no back-fill is initialized - run `swap-backfill init` first")
	}

	cp, err := s.repo.GetCheckpoint()
	if err != nil {
		return nil, err
	}

	if cp == nil {
		return nil, fmt.Errorf("the staging tables exist but hold no checkpoint - run `swap-backfill cleanup` and start over")
	}

	return cp, nil
}

// dayStart floors a timestamp to the biggest candle bucket. Committing one day
// at a time keeps every candle length window-complete inside a single
// transaction.
func dayStart(t time.Time) time.Time {
	start, _ := interval.GetTimestampInterval(t.Unix(), interval.GetBiggestDuration())

	return start
}
