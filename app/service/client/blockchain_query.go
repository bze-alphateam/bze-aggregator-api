package client

import (
	"encoding/json"
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"io/ioutil"
	"net/http"
	"strconv"
)

const (
	supplyPath        = "/cosmos/bank/v1beta1/supply"
	communityPoolPath = "/cosmos/distribution/v1beta1/community_pool"

	denom = "ubze"
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
		return nil, fmt.Errorf("blockchain host is empty")
	}

	return &BlockchainQueryClient{host}, nil
}

func (c *BlockchainQueryClient) GetTotalSupply() (int64, error) {
	url := fmt.Sprintf("%s%s", c.Host, supplyPath)
	resp, err := http.Get(url)
	if err != nil {

		return 0, fmt.Errorf("error making request to Cosmos SDK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		return 0, fmt.Errorf("received non-OK status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {

		return 0, fmt.Errorf("error reading response body: %w", err)
	}

	var data supplyResponse
	err = json.Unmarshal(body, &data)
	if err != nil {

		return 0, fmt.Errorf("error unmarshalling response data: %w", err)
	}

	for _, amt := range data.Amount {
		if amt.Denom == denom {
			totalSupply, err := strconv.ParseInt(amt.Amount, 10, 64)
			if err != nil {

				return 0, fmt.Errorf("error parsing total supply: %w", err)
			}

			return totalSupply, nil
		}
	}

	return 0, fmt.Errorf("denom %s not found", denom)
}

func (c *BlockchainQueryClient) GetCommunityPoolTotal() (float64, error) {
	url := fmt.Sprintf("%s%s", c.Host, communityPoolPath)
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("error making request to Cosmos SDK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("received non-OK status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
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
