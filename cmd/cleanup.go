package cmd

import (
	"fmt"

	"github.com/bze-alphateam/bze-aggregator-api/cmd/factory"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/spf13/cobra"
)

const (
	flagDays = "days"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Args:  cobra.ExactArgs(0),
	Short: "Cleanup old blockchain data",
	Long: `Cleanup old blockchain data from the database based on age.
This command deletes blocks and related data (tx_results, events, attributes)
that are older than the specified number of days.

Usage:
./bze-agg cleanup --days 30
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		days, err := cmd.Flags().GetInt(flagDays)
		if err != nil {
			return fmt.Errorf("error parsing days flag: %w", err)
		}

		if days <= 0 {
			return fmt.Errorf("days must be a positive integer, got: %d", days)
		}

		cfg, err := config.NewAppConfig()
		if err != nil {
			return err
		}

		logger, err := internal.NewLogger(cfg)
		if err != nil {
			return err
		}
		logger = logger.WithField("command", "cleanup")

		handler, err := factory.GetCleanupHandler(logger)
		if err != nil {
			return err
		}

		return handler.CleanupOldBlocks(days)
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().Int(flagDays, 0, "Number of days to retain (blocks older than this will be deleted)")
	_ = cleanupCmd.MarkFlagRequired(flagDays)
}
