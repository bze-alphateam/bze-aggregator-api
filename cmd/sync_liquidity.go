package cmd

import (
	"github.com/bze-alphateam/bze-aggregator-api/cmd/factory"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/spf13/cobra"
)

var syncLiquidityCmd = &cobra.Command{
	Use:   "liquidity",
	Args:  cobra.ExactArgs(0),
	Short: "Sync liquidity pools",
	Long: `Sync liquidity pools from blockchain into the database
Usage:
./bze-agg sync liquidity
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
		logger = logger.WithField("command", "sync_liquidity")

		handler, err := factory.GetLiquidityPoolSyncHandler(cfg, logger)
		if err != nil {
			return err
		}

		handler.SyncLiquidityPools()

		return nil
	},
}

func init() {
	syncCmd.AddCommand(syncLiquidityCmd)
}
