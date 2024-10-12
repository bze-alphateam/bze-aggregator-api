package main

import (
	"github.com/bze-alphateam/bze-aggregator-api/cmd"
	"github.com/bze-alphateam/bze-aggregator-api/server"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		cmd.Execute()
	} else {
		server.Start()
	}
}
