package cmd

import (
	"github.com/bze-alphateam/bze-aggregator-api/cmd/factory"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/spf13/cobra"
)

const (
	flagMarketId = "market-id"
)

var syncOrdersCmd = &cobra.Command{
	Use:   "orders",
	Args:  cobra.ExactArgs(0),
	Short: "Sync active orders",
	Long: `Sync active orders for a market or more
Usage:
./bze-agg sync orders
./bze-agg sync orders --market-id "uvdl/ubze"
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

		handler, err := factory.GetMarketOrderSyncHandler(cfg, logger)
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

			return handler.SyncMarketOrders(marketId)
		}

		return nil
	},
}

func init() {
	syncCmd.AddCommand(syncOrdersCmd)
	syncOrdersCmd.Flags().String(flagMarketId, "", "the blockchain market id we want to sync")
}
