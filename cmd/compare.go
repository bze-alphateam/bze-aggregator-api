package cmd

import (
	"os"

	"github.com/bze-alphateam/bze-aggregator-api/app/service/compare"
)

func init() {
	rootCmd.AddCommand(compare.NewCommand(compare.NewHTTPClient(), compare.SystemClock{}, os.Stdout))
}
