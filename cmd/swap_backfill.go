package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/bze-alphateam/bze-aggregator-api/app/service/backfill"
	"github.com/bze-alphateam/bze-aggregator-api/cmd/factory"
	"github.com/bze-alphateam/bze-aggregator-api/cmd/handlers"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/spf13/cobra"
)

const (
	flagNode      = "node"
	flagWorkers   = "workers"
	flagChunkSize = "chunk-size"
	flagForce     = "force"
)

var swapBackfillCmd = &cobra.Command{
	Use:   "swap-backfill",
	Short: "Recover liquidity pool swap history from an archive node",
	Long: `Recover liquidity pool swap history from an archive node.

LP swaps are not part of chain state: they only exist as emitted events, which
are normally ingested from the node's Postgres event index. When that index is
lost this command walks historical blocks and parses the swaps back out of them.

It is a disaster-recovery one-shot, not part of normal operation, and it is
built to run next to a working listener. The scan stages everything in temp
tables; nothing touches the live tables until "commit" is confirmed.

Usage:
./bze-agg swap-backfill init                 # ask where the data stops, then set the range
./bze-agg swap-backfill run --node <rpc>     # scan the range (resumable, may take days)
./bze-agg swap-backfill commit               # move the staged swaps into the live tables
./bze-agg swap-backfill cleanup              # discard an unwanted back-fill
`,
}

var swapBackfillInitCmd = &cobra.Command{
	Use:   "init [block_height]",
	Args:  cobra.MaximumNArgs(1),
	Short: "Prepare a back-fill and pick the height to start from",
	Long: `Create the staging tables and record the range to scan.

Without a block height the command reports the oldest swap it still holds for
each pool - everything before that is what is missing - and then asks for the
height to start from. Overshooting is safe.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var height *int64
		if len(args) == 1 {
			parsed, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("%q is not a block height", args[0])
			}

			height = &parsed
		}

		handler, err := newSwapBackfillHandler(cmd)
		if err != nil {
			return err
		}

		return handler.Init(height)
	},
}

var swapBackfillRunCmd = &cobra.Command{
	Use:   "run",
	Args:  cobra.ExactArgs(0),
	Short: "Scan the initialized range and stage every swap found",
	Long: `Walk the initialized range in descending order and stage the swaps found.

The checkpoint advances with every chunk, so the command can be interrupted and
re-run as often as needed - it always continues where it stopped. Point it at an
archive node with --node so the listener's own node is left alone.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workers, err := cmd.Flags().GetInt(flagWorkers)
		if err != nil {
			return err
		}

		chunkSize, err := cmd.Flags().GetInt(flagChunkSize)
		if err != nil {
			return err
		}

		handler, err := newSwapBackfillHandler(cmd)
		if err != nil {
			return err
		}

		//stop between chunks on ctrl-c/SIGTERM instead of losing the current chunk
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		return handler.Run(ctx, workers, chunkSize)
	},
}

var swapBackfillCommitCmd = &cobra.Command{
	Use:   "commit",
	Args:  cobra.ExactArgs(0),
	Short: "Move the staged swaps into market_history and build their candles",
	Long: `Insert the staged swaps into the live tables, one day at a time.

Prints what it is about to do and asks for confirmation. Rows the listener has
already ingested are skipped, and the day the live data starts in is left for
the listener to build its candles from.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		handler, err := newSwapBackfillHandler(cmd)
		if err != nil {
			return err
		}

		return handler.Commit()
	},
}

var swapBackfillCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Args:  cobra.ExactArgs(0),
	Short: "Drop the staging tables",
	Long: `Discard a back-fill: prints what is staged and drops both tables.

Refuses once anything has been committed - the staging tables are then the only
record of what was inserted into market_history, and losing that record makes a
second run duplicate everything undetectably.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, err := cmd.Flags().GetBool(flagForce)
		if err != nil {
			return err
		}

		handler, err := newSwapBackfillHandler(cmd)
		if err != nil {
			return err
		}

		return handler.Cleanup(force)
	},
}

func newSwapBackfillHandler(cmd *cobra.Command) (*handlers.SwapBackfill, error) {
	cfg, err := config.NewAppConfig()
	if err != nil {
		return nil, err
	}

	logger, err := internal.NewLogger(cfg)
	if err != nil {
		return nil, err
	}
	logger = logger.WithField("command", "swap_backfill")

	node, err := cmd.Flags().GetString(flagNode)
	if err != nil {
		return nil, err
	}

	return factory.GetSwapBackfillHandler(cfg, logger, node)
}

func init() {
	rootCmd.AddCommand(swapBackfillCmd)
	swapBackfillCmd.PersistentFlags().String(flagNode, "", "RPC host to read blocks from (default: BLOCKCHAIN_RPC_HOST). Use an archive node")

	swapBackfillCmd.AddCommand(swapBackfillInitCmd)
	swapBackfillCmd.AddCommand(swapBackfillRunCmd)
	swapBackfillCmd.AddCommand(swapBackfillCommitCmd)
	swapBackfillCmd.AddCommand(swapBackfillCleanupCmd)

	swapBackfillRunCmd.Flags().Int(flagWorkers, backfill.DefaultWorkers, "How many block requests to keep in flight")
	swapBackfillRunCmd.Flags().Int(flagChunkSize, backfill.DefaultChunkSize, "How many heights to scan before the checkpoint advances")
	swapBackfillCleanupCmd.Flags().Bool(flagForce, false, "Drop the tables even when rows were already committed (dangerous)")
}
