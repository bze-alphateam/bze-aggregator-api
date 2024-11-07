package cmd

import (
	"github.com/bze-alphateam/bze-aggregator-api/cmd/factory"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	"github.com/spf13/cobra"
)

var syncListenerCmd = &cobra.Command{
	Use:   "listener",
	Args:  cobra.ExactArgs(0),
	Short: "Sync listener",
	Long: `Sync listener subscribes to tendermint websocket and listens for DEX changes and syncs them
Usage:
./bze-agg sync listener
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

		handler, err := factory.GetSyncListener(cfg, logger)
		if err != nil {
			return err
		}

		return handler.ListenAndSync()
	},
}

func init() {
	syncCmd.AddCommand(syncListenerCmd)
}
