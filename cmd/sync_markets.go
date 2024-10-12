package cmd

import (
	"github.com/bze-alphateam/bze-aggregator-api/cmd/factory"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/spf13/cobra"
)

var syncMarketsCmd = &cobra.Command{
	Use:   "markets",
	Short: "Sync available markets",
	Long:  `Sync available markets from blockchain into the database`,
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
	syncCmd.AddCommand(syncMarketsCmd)
}
