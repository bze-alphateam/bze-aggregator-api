package cmd

import (
	"github.com/bze-alphateam/bze-aggregator-api/cmd/factory"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/spf13/cobra"
)

var syncIntervalsCmd = &cobra.Command{
	Use:   "intervals",
	Args:  cobra.ExactArgs(0),
	Short: "Sync active orders",
	Long: `Sync active orders for a market or more
Usage:
./bze-agg sync intervals
./bze-agg sync intervals --market-id "uvdl/ubze"
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

		handler, err := factory.GetMarketIntervalSyncHandler(cfg, logger)
		if err != nil {
			return err
		}

		marketId, _ := cmd.Flags().GetString(flagMarketId)
		if marketId == "" {
			logger.Info("no market id specified")
			logger.Info("syncing all markets intervals")

			handler.SyncAll()
		} else {
			logger.Infof("syncing intervals for market with id %s", marketId)

			return handler.SyncIntervals(marketId)
		}

		return nil
	},
}

func init() {
	syncCmd.AddCommand(syncIntervalsCmd)
}
