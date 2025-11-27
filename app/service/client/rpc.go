package client

import (
	"github.com/cometbft/cometbft/rpc/client/http"
)

func GetRpcClient(host string) (client *http.HTTP, err error) {
	return http.New(host, endpoint)
}
