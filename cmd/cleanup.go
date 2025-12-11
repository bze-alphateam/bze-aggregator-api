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
	flagAll  = "all"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Args:  cobra.ExactArgs(0),
	Short: "Cleanup old blockchain data",
	Long: `Cleanup old blockchain data from the database based on age.

By default, this command only deletes blocks that don't have events with type starting with "bze.".
Use the --all flag to delete ALL old blocks regardless of their events.

Usage:
./bze-agg cleanup --days 30                    # Delete only blocks without "bze." events
./bze-agg cleanup --days 30 --all              # Delete ALL old blocks
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		days, err := cmd.Flags().GetInt(flagDays)
		if err != nil {
			return fmt.Errorf("error parsing days flag: %w", err)
		}

		if days <= 0 {
			return fmt.Errorf("days must be a positive integer, got: %d", days)
		}

		deleteAll, err := cmd.Flags().GetBool(flagAll)
		if err != nil {
			return fmt.Errorf("error parsing all flag: %w", err)
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

		return handler.CleanupOldBlocks(days, deleteAll)
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().Int(flagDays, 0, "Number of days to retain (blocks older than this will be deleted)")
	cleanupCmd.Flags().Bool(flagAll, false, "Delete ALL old blocks regardless of events (default: only blocks without 'bze.' events)")
	_ = cleanupCmd.MarkFlagRequired(flagDays)
}
