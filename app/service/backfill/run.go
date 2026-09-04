package backfill

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
)

const (
	DefaultWorkers   = 4
	DefaultChunkSize = 1000

	maxWorkers   = 32
	maxChunkSize = 10000

	rpcAttempts   = 3
	rpcRetryDelay = time.Second
)

type RunOptions struct {
	// Workers bounds how many block_results calls are in flight at once. Size
	// it for the archive node's comfort, not for speed.
	Workers int
	// ChunkSize is how many heights are scanned before the checkpoint advances.
	// Smaller chunks lose less work on an interruption and hold less in memory.
	ChunkSize int
}

// Run walks the blocks of the initialized range in descending order and stages
// every swap it finds. It resumes from the checkpoint, so it can be interrupted
// and restarted with the same command as often as needed.
func (s *Service) Run(ctx context.Context, opts RunOptions) error {
	cp, err := s.requireCheckpoint()
	if err != nil {
		return err
	}

	if cp.ScanComplete {
		s.logger.Infof("scan already complete (%d down to %d) - run `swap-backfill commit` next", cp.InitHeight, cp.FloorHeight)

		return nil
	}

	workers := opts.Workers
	if workers <= 0 || workers > maxWorkers {
		return fmt.Errorf("workers must be between 1 and %d", maxWorkers)
	}

	chunkSize := opts.ChunkSize
	if chunkSize <= 0 || chunkSize > maxChunkSize {
		return fmt.Errorf("chunk-size must be between 1 and %d", maxChunkSize)
	}

	s.logger.Infof("scanning from %d down to %d with %d workers", cp.NextHeight, cp.FloorHeight, workers)

	started := time.Now()
	var scanned, staged int64
	for {
		if ctx.Err() != nil {
			s.logger.Infof("stopped at height %d - re-run the same command to continue", cp.NextHeight)

			return nil
		}

		high, low, done := chunkRange(cp, chunkSize)
		if done {
			// nothing left to scan: mark the scan complete and stop
			if err = s.repo.SaveScanChunk(nil, cp.NextHeight, true); err != nil {
				return err
			}

			break
		}

		rows, err := s.scanRange(ctx, low, high, workers)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				s.logger.Infof("stopped at height %d - re-run the same command to continue", cp.NextHeight)

				return nil
			}

			return err
		}

		complete := low == cp.FloorHeight
		if err = s.repo.SaveScanChunk(rows, low-1, complete); err != nil {
			return err
		}

		cp.NextHeight = low - 1
		cp.ScanComplete = complete
		scanned += high - low + 1
		staged += int64(len(rows))

		s.logProgress(cp, started, scanned, staged, len(rows))

		if complete {
			break
		}
	}

	s.logger.Infof("scan complete: %d blocks scanned, %d swaps staged - review them and run `swap-backfill commit`", scanned, staged)

	return nil
}

// chunkRange returns the next descending chunk of heights to scan. The bounds
// of the initialized range are hard: a different range means a fresh init.
func chunkRange(cp *entity.BackfillCheckpoint, chunkSize int) (high, low int64, done bool) {
	high = cp.NextHeight
	if high > cp.InitHeight {
		high = cp.InitHeight
	}

	if high < cp.FloorHeight {
		return 0, 0, true
	}

	low = high - int64(chunkSize) + 1
	if low < cp.FloorHeight {
		low = cp.FloorHeight
	}

	return high, low, false
}

// scanRange fetches the block results of a height range concurrently and turns
// the swap events it finds into staged rows.
func (s *Service) scanRange(ctx context.Context, low, high int64, workers int) ([]*entity.StagedSwap, error) {
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		firstErr error
	)

	found := make(map[int64][]rawSwapEvent)
	sem := make(chan struct{}, workers)

	for height := high; height >= low; height-- {
		mu.Lock()
		failed := firstErr != nil
		mu.Unlock()
		if failed || ctx.Err() != nil {
			break
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(h int64) {
			defer wg.Done()
			defer func() { <-sem }()

			res, err := s.fetchBlockResults(ctx, h)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()

				return
			}

			events := extractSwapEvents(h, res)
			if len(events) == 0 {
				return
			}

			mu.Lock()
			found[h] = events
			mu.Unlock()
		}(height)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return s.convertFound(found)
}

// convertFound parses the events of the blocks that actually contained swaps.
// This is done outside the fan-out on purpose: only a tiny fraction of blocks
// hold swaps, and the block time these need is fetched over RPC as well.
func (s *Service) convertFound(found map[int64][]rawSwapEvent) ([]*entity.StagedSwap, error) {
	heights := make([]int64, 0, len(found))
	for height := range found {
		heights = append(heights, height)
	}
	sort.Slice(heights, func(i, j int) bool { return heights[i] > heights[j] })

	var rows []*entity.StagedSwap
	for _, height := range heights {
		executedAt, err := s.chain.GetBlockTime(height)
		if err != nil {
			return nil, fmt.Errorf("could not get the time of block %d: %w", height, err)
		}

		for _, raw := range found[height] {
			row, err := toStagedSwap(s.assets, raw, executedAt)
			if err != nil {
				// a swap we cannot parse is history we would silently lose, so
				// stop and let the operator look at it
				return nil, fmt.Errorf("could not convert the swap event at height %d (tx %d, event %d): %w", height, raw.TxIndex, raw.EventIndex, err)
			}

			rows = append(rows, row)
		}
	}

	return rows, nil
}

func (s *Service) fetchBlockResults(ctx context.Context, height int64) (*coretypes.ResultBlockResults, error) {
	var lastErr error
	for attempt := 1; attempt <= rpcAttempts; attempt++ {
		res, err := s.chain.GetBlockResults(ctx, height)
		if err == nil {
			return res, nil
		}

		lastErr = err
		s.logger.WithError(err).Warnf("block results request failed for height %d (attempt %d/%d)", height, attempt, rpcAttempts)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * rpcRetryDelay):
		}
	}

	return nil, fmt.Errorf("could not get block results for height %d: %w", height, lastErr)
}

func (s *Service) logProgress(cp *entity.BackfillCheckpoint, started time.Time, scanned, staged int64, chunkRows int) {
	remaining := cp.NextHeight - cp.FloorHeight + 1
	if remaining < 0 {
		remaining = 0
	}

	elapsed := time.Since(started).Seconds()
	rate := float64(scanned) / elapsed
	eta := "unknown"
	if rate > 0 {
		eta = (time.Duration(float64(remaining)/rate) * time.Second).Round(time.Second).String()
	}

	s.logger.Infof(
		"at height %d: %d blocks scanned (%.1f blocks/s), %d swaps staged (%d in this chunk), %d blocks left, eta %s",
		cp.NextHeight, scanned, rate, staged, chunkRows, remaining, eta,
	)
}
