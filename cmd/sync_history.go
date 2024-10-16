package cmd

import (
	"github.com/bze-alphateam/bze-aggregator-api/cmd/factory"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/spf13/cobra"
)

var syncHistoryCmd = &cobra.Command{
	Use:   "history",
	Args:  cobra.ExactArgs(0),
	Short: "Sync history orders",
	Long: `Sync history orders for a market or more
Usage:
./bze-agg sync history
./bze-agg sync history --market-id "uvdl/ubze"
`,
	RunE: func(cmd *cobra.Command, args []string) error {

		cfg, err := config.NewAppConfig()
		if err != nil {
			return err
		}

		logger, err := internal.NewLogger(cfg)
		if err != nil {
			return err
		}

		handler, err := factory.GetMarketHistorySyncHandler(cfg, logger)
		if err != nil {
			return err
		}

		marketId, _ := cmd.Flags().GetString(flagMarketId)
		if marketId == "" {
			logger.Info("no market id specified")
			logger.Info("syncing all markets orders")

			handler.SyncAll()
		} else {
			logger.Infof("syncing orders for market with id %s", marketId)

			return handler.SyncHistory(marketId)
		}

		return nil
	},
}

func init() {
	syncCmd.AddCommand(syncHistoryCmd)
}
