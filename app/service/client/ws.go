package client

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/tendermint/tendermint/rpc/client/http"
)

const (
	endpoint = "/websocket"
)

var wsClient *http.HTTP

func GetWsClient() (*http.HTTP, error) {
	if wsClient == nil {
		wsEnv, err := getHost()
		if err != nil {
			return nil, err
		}

		client, err := http.New(wsEnv, endpoint)
		if err != nil {
			return nil, err
		}

		wsClient = client
	}

	return wsClient, nil
}

func getHost() (string, error) {
	envFile, err := godotenv.Read(".env")
	if err != nil {
		return "", err
	}

	wsEnv, ok := envFile["BLOCKCHAIN_WS_HOST"]
	if !ok {
		return "", fmt.Errorf("BLOCKCHAIN_WS_HOST not found in .env")
	}

	if wsEnv == "" {
		return "", fmt.Errorf("BLOCKCHAIN_WS_HOST is empty")
	}

	return wsEnv, nil
}
