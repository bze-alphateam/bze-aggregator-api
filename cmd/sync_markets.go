package cmd

import "github.com/spf13/cobra"

var syncMarketsCmd = &cobra.Command{
	Use:   "sync markets",
	Short: "Sync available markets",
	Long:  `Sync available markets from blockchain into the database`,
	RunE: func(cmd *cobra.Command, args []string) error {

		return nil
	},
}

func init() {
	syncCmd.AddCommand(syncMarketsCmd)
}
