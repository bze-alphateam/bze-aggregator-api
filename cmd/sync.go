package cmd

import "github.com/spf13/cobra"

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync all markets & data",
	Long: `List of sync commands:
Usage:
./bze-agg sync markets
./bze-agg sync orders
`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Usage()
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
