package handlers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/service/backfill"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
)

type backfillService interface {
	InitStatus() (*backfill.InitStatus, error)
	Init(height int64) error
	Status() (*backfill.Status, error)
	Cleanup(force bool) error
	Run(ctx context.Context, opts backfill.RunOptions) error
	CommitPlan() (*backfill.CommitPlan, error)
	Commit(plan *backfill.CommitPlan) (*backfill.CommitResult, error)
}

type SwapBackfill struct {
	logger logrus.FieldLogger
	svc    backfillService

	in  *bufio.Reader
	out io.Writer
}

func NewSwapBackfillHandler(logger logrus.FieldLogger, svc backfillService) (*SwapBackfill, error) {
	if logger == nil || svc == nil {
		return nil, internal.NewInvalidDependenciesErr("NewSwapBackfillHandler")
	}

	return &SwapBackfill{logger: logger, svc: svc, in: bufio.NewReader(os.Stdin), out: os.Stdout}, nil
}

// Init prepares a back-fill. When no height is given it asks for one, after
// showing where the data actually stops - the operator has no other way of
// knowing which block to start from.
func (s *SwapBackfill) Init(height *int64) error {
	status, err := s.svc.InitStatus()
	if err != nil {
		return err
	}

	if status.Initialized {
		s.printStatus(&backfill.Status{Checkpoint: status.Checkpoint, Counts: status.Counts})

		return fmt.Errorf("a back-fill is already initialized - review the staged data, continue it with `swap-backfill run`, or discard it with `swap-backfill cleanup`")
	}

	s.printBoundaries(status)

	if height == nil {
		entered, err := s.askHeight()
		if err != nil {
			return err
		}

		height = &entered
	}

	if err = s.svc.Init(*height); err != nil {
		return err
	}

	s.printf("\nInitialized. Scan the range with:\n  bze-agg swap-backfill run --node <archive-rpc>\n")

	return nil
}

func (s *SwapBackfill) Run(ctx context.Context, workers, chunkSize int) error {
	return s.svc.Run(ctx, backfill.RunOptions{Workers: workers, ChunkSize: chunkSize})
}

// Cleanup discards a back-fill after showing exactly what is being thrown away.
func (s *SwapBackfill) Cleanup(force bool) error {
	status, err := s.svc.Status()
	if err != nil {
		return err
	}

	s.printStatus(status)
	s.printf("\nThis drops both staging tables. Everything staged and not yet committed is lost.\n")

	confirmed, err := s.confirm()
	if err != nil {
		return err
	}

	if !confirmed {
		s.printf("Aborted.\n")

		return nil
	}

	return s.svc.Cleanup(force)
}

// Commit inserts the staged swaps into the live tables after showing the plan.
func (s *SwapBackfill) Commit() error {
	plan, err := s.svc.CommitPlan()
	if err != nil {
		return err
	}

	s.printCommitPlan(plan)

	confirmed, err := s.confirm()
	if err != nil {
		return err
	}

	if !confirmed {
		s.printf("Aborted.\n")

		return nil
	}

	result, err := s.svc.Commit(plan)
	if err != nil {
		return err
	}

	s.printf("\nCommitted %d rows over %d days, wrote %d candles.\n", result.Rows, result.Days, result.Intervals)
	if result.DeferredRows > 0 {
		s.printf("%d rows were left with i_added_to_interval = 0: the listener owns those candles and rebuilds them on its next sync for the pool.\n", result.DeferredRows)
	}

	for _, market := range result.Markets {
		s.printf("Market %s now starts at %s.\n", market.MarketID, market.CreatedAt.Format(time.RFC3339))
	}

	s.printf("Staging tables kept as %s / %s - they are the record of what was inserted.\n", result.CheckpointTable, result.StagedTable)

	return nil
}

func (s *SwapBackfill) printBoundaries(status *backfill.InitStatus) {
	s.printf("Oldest LP swap still held per pool:\n\n")

	w := tabwriter.NewWriter(s.out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "POOL\tOLDEST SWAP HELD")
	for _, pool := range status.Pools {
		value := "no data"
		if pool.OldestSwap != nil {
			value = pool.OldestSwap.Format(time.RFC3339)
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\n", pool.MarketID, value)
	}
	_ = w.Flush()

	if status.Boundary == nil {
		s.printf("\nNo pool holds any swap at all: everything is missing.\n")
	} else {
		s.printf("\nData is missing before %s (the lowest of the above).\n", status.Boundary.Format(time.RFC3339))
	}

	s.printf("Look up a block height at or just before that time on the archive node.\n")
	s.printf("Overshooting is safe - anything already ingested is dropped at commit time.\n")
	s.printf("Allowed range: %d (liquidity pools upgrade) to %d (current chain height).\n", status.FloorHeight, status.CurrentHeight)
	s.printf("In doubt, the current chain height is always a valid answer - it only means scanning more blocks.\n")
}

func (s *SwapBackfill) printStatus(status *backfill.Status) {
	cp := status.Checkpoint
	if cp == nil {
		s.printf("\nThe staging tables exist but hold no checkpoint - the back-fill is broken.\n")
		s.printf("  staged: %d rows (%d committed, %d uncommitted)\n", status.Counts.Total, status.Counts.Committed, status.Counts.Uncommitted())

		return
	}

	scanned := cp.InitHeight - cp.NextHeight
	if scanned < 0 {
		scanned = 0
	}

	s.printf("\nBack-fill initialized at %s\n", cp.CreatedAt.Format(time.RFC3339))
	s.printf("  range:      %d down to %d\n", cp.InitHeight, cp.FloorHeight)
	s.printf("  next block: %d (%d blocks scanned, scan complete: %t)\n", cp.NextHeight, scanned, cp.ScanComplete)
	s.printf("  staged:     %d rows (%d committed, %d uncommitted)\n", status.Counts.Total, status.Counts.Committed, status.Counts.Uncommitted())
}

func (s *SwapBackfill) printCommitPlan(plan *backfill.CommitPlan) {
	s.printStatus(&backfill.Status{Checkpoint: plan.Checkpoint, Counts: plan.Counts})
	s.printf("\nWhat will be committed:\n\n")

	w := tabwriter.NewWriter(s.out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "POOL\tROWS\tFROM\tTO\tSKIPPED (already held)")
	for _, market := range plan.Markets {
		_, _ = fmt.Fprintf(
			w, "%s\t%d\t%s\t%s\t%s\n",
			market.MarketID,
			market.Eligible.Count,
			formatNullTime(market.Eligible.MinExecutedAt.Valid, market.Eligible.MinExecutedAt.Time),
			formatNullTime(market.Eligible.MaxExecutedAt.Valid, market.Eligible.MaxExecutedAt.Time),
			formatSkipped(market),
		)
	}
	_ = w.Flush()

	if plan.Eligible == 0 {
		s.printf("\nNothing left to insert. The commit will only re-date the markets and archive the staging tables.\n")

		return
	}

	s.printf("\n%d rows will be inserted over %s .. %s, %d rows are skipped as already held.\n",
		plan.Eligible, plan.FirstDay.Format(time.DateOnly), plan.LastDay.Format(time.DateOnly), plan.Skipped)
	s.printf("Each day is one transaction: rows, candles and the committed flag land together, so an interrupted commit can simply be re-run.\n")
}

func formatSkipped(market backfill.MarketPlan) string {
	if market.LiveFloor == nil {
		return "no live data"
	}

	if market.Skipped.Count == 0 {
		return "0"
	}

	return fmt.Sprintf("%d (heights %d-%d, from %s)",
		market.Skipped.Count,
		market.Skipped.MinHeight.Int64,
		market.Skipped.MaxHeight.Int64,
		market.LiveFloor.Format(time.RFC3339),
	)
}

func formatNullTime(valid bool, value time.Time) string {
	if !valid {
		return "-"
	}

	return value.Format(time.RFC3339)
}

func (s *SwapBackfill) askHeight() (int64, error) {
	s.printf("\nStart height: ")
	line, err := s.readLine()
	if err != nil {
		return 0, err
	}

	height, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a block height", line)
	}

	return height, nil
}

func (s *SwapBackfill) confirm() (bool, error) {
	s.printf("\nType 'yes' to continue: ")
	line, err := s.readLine()
	if err != nil {
		return false, err
	}

	return strings.EqualFold(line, "yes"), nil
}

func (s *SwapBackfill) readLine() (string, error) {
	line, err := s.in.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("could not read the answer (this command needs an interactive terminal - run it with `docker compose run --rm -it`): %w", err)
	}

	return strings.TrimSpace(line), nil
}

func (s *SwapBackfill) printf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(s.out, format, args...)
}
