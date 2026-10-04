package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
)

const (
	supplyPath        = "/cosmos/bank/v1beta1/supply"
	communityPoolPath = "/cosmos/distribution/v1beta1/community_pool"
	marketHistoryPath = "/bze/tradebin/market_history"
	latestBlockPath   = "/cosmos/base/tendermint/v1beta1/blocks/latest"

	// supplyPageLimit is sent as pagination.limit on supplyPath: the node pages
	// at 100 by default and GetTotalSupply only reads one page.
	supplyPageLimit = 5000
)

type supplyResponse struct {
	Amount []dto.Coin `json:"supply"`
}

type communityPoolResponse struct {
	Pool []dto.Coin `json:"pool"`
}

type BlockchainQueryClient struct {
	Host string
}

func NewBlockchainQueryClient(host string) (*BlockchainQueryClient, error) {
	if len(host) == 0 {
		return nil, internal.NewInvalidDependenciesErr("NewBlockchainQueryClient")
	}

	return &BlockchainQueryClient{host}, nil
}

func (c *BlockchainQueryClient) GetTotalSupply(denom string) (math.Int, error) {
	url := fmt.Sprintf("%s%s?pagination.limit=%d", c.Host, supplyPath, supplyPageLimit)
	resp, err := http.Get(url)
	if err != nil {

		return math.Int{}, fmt.Errorf("error making request to Cosmos SDK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		return math.Int{}, fmt.Errorf("received non-OK status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {

		return math.Int{}, fmt.Errorf("error reading response body: %w", err)
	}

	var data supplyResponse
	err = json.Unmarshal(body, &data)
	if err != nil {

		return math.Int{}, fmt.Errorf("error unmarshalling response data: %w", err)
	}

	for _, amt := range data.Amount {
		if amt.Denom == denom {
			totalSupply, ok := math.NewIntFromString(amt.Amount)
			if !ok {

				return math.Int{}, fmt.Errorf("error parsing total supply: %q", amt.Amount)
			}

			return totalSupply, nil
		}
	}

	return math.Int{}, fmt.Errorf("denom %s not found", denom)
}

func (c *BlockchainQueryClient) GetCommunityPoolTotal(denom string) (float64, error) {
	url := fmt.Sprintf("%s%s", c.Host, communityPoolPath)
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("error making request to Cosmos SDK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("received non-OK status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %w", err)
	}

	var data communityPoolResponse
	err = json.Unmarshal(body, &data)
	if err != nil {
		return 0, fmt.Errorf("error unmarshalling response data: %w", err)
	}

	for _, amt := range data.Pool {
		if amt.Denom == denom {
			totalAmount, err := strconv.ParseFloat(amt.Amount, 64)
			if err != nil {
				return 0, fmt.Errorf("error parsing community pool total: %w", err)
			}
			return totalAmount, nil
		}
	}

	return 0, fmt.Errorf("denom %s not found in community pool", denom)
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

func (c *BlockchainQueryClient) GetLatestBlock() (*coretypes.ResultBlock, error) {
	url := fmt.Sprintf("%s%s", c.Host, latestBlockPath)
	resp, err := http.Get(url)
	if err != nil {

		return nil, fmt.Errorf("error making request to Cosmos SDK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		return nil, fmt.Errorf("received non-OK status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {

		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	fmt.Println(string(body))

	var data coretypes.ResultBlock
	err = cmtjson.Unmarshal(body, &data)
	if err != nil {

		return nil, fmt.Errorf("error unmarshalling response data: %w", err)
	}

	return &data, nil
}
