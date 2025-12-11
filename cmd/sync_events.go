package cmd

import (
	"fmt"

	"github.com/bze-alphateam/bze-aggregator-api/cmd/factory"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/spf13/cobra"
)

const (
	flagBatchSize = "batch-size"
)

var syncEventsCmd = &cobra.Command{
	Use:   "events",
	Args:  cobra.ExactArgs(0),
	Short: "Sync swap events from PostgreSQL to MySQL",
	Long: `Sync unprocessed swap events from PostgreSQL database to MySQL market_history table.
This command processes SwapEvent entries and creates corresponding market history records.

Usage:
./bze-agg sync events
./bze-agg sync events --batch-size 100
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		batchSize, err := cmd.Flags().GetInt(flagBatchSize)
		if err != nil {
			return err
		}
		if batchSize <= 0 || batchSize > 10000 {
			return fmt.Errorf("batch-size must be a positive integer between 1 and 10000")
		}

		cfg, err := config.NewAppConfig()
		if err != nil {
			return err
		}

		logger, err := internal.NewLogger(cfg)
		if err != nil {
			return err
		}
		logger = logger.WithField("command", "sync_events")

		handler, err := factory.GetSyncEventsHandler(cfg, logger)
		if err != nil {
			return err
		}

		return handler.SyncSwapEvents(batchSize)
	},
}

func init() {
	syncCmd.AddCommand(syncEventsCmd)
	syncEventsCmd.Flags().Int(flagBatchSize, 1000, "Number of events to process in one batch")
}
