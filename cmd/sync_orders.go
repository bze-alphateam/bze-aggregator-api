package cmd

import (
	"github.com/bze-alphateam/bze-aggregator-api/cmd/factory"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/spf13/cobra"
)

var syncOrdersCmd = &cobra.Command{
	Use:   "orders",
	Short: "Sync active orders",
	Long:  `Sync active orders for a market or more`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.NewAppConfig()
		if err != nil {
			return err
		}

		logger, err := internal.NewLogger(cfg)
		if err != nil {
			return err
		}

		handler, err := factory.GetMarketsSyncHandler(cfg, logger)
		if err != nil {
			return err
		}

		handler.SyncMarkets()

		return nil
	},
}

func init() {
	syncCmd.AddCommand(syncOrdersCmd)
}
