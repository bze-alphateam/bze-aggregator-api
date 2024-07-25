package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

const (
	BasePath = "/cosmos/bank/v1beta1/supply"
	Denom    = "ubze"
)

type supplyResponse struct {
	Amount []struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"supply"`
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
	url := fmt.Sprintf("%s%s", c.Host, BasePath)
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
		if amt.Denom == Denom {
			totalSupply, err := strconv.ParseInt(amt.Amount, 10, 64)
			if err != nil {

				return 0, fmt.Errorf("error parsing total supply: %w", err)
			}

			return totalSupply, nil
		}
	}

	return 0, fmt.Errorf("denom %s not found", Denom)
}
