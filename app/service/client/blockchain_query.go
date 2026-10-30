package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
)

const (
	marketHistoryPath = "/bze/tradebin/market_history"
)

type BlockchainQueryClient struct {
	Host string
}

func NewBlockchainQueryClient(host string) (*BlockchainQueryClient, error) {
	if len(host) == 0 {
		return nil, internal.NewInvalidDependenciesErr("NewBlockchainQueryClient")
	}

	return &BlockchainQueryClient{host}, nil
}

func (c *BlockchainQueryClient) GetMarketHistory(marketId string, limit int) ([]dto.HistoryOrder, error) {
	url := c.getMarketHistoryUrl(marketId, limit)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error making request to the blockchain: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-OK status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var data dto.HistoryResponse
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response data: %w", err)
	}

	return data.List, nil
}

func (c *BlockchainQueryClient) getMarketHistoryUrl(marketId string, limit int) string {
	return fmt.Sprintf("%s%s?market=%s&pagination.limit=%d&pagination.reverse=true", c.Host, marketHistoryPath, marketId, limit)
}
