package client

import (
	"github.com/tendermint/tendermint/rpc/client/http"
)

func GetRpcClient(host string) (client *http.HTTP, err error) {
	return http.New(host, endpoint)
}
