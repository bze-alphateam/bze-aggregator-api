package client

import (
	"github.com/cometbft/cometbft/rpc/client/http"
)

const endpoint = "/websocket"

func GetRpcClient(host string) (client *http.HTTP, err error) {
	return http.New(host, endpoint)
}
