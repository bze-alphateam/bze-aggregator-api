package client

import (
	"encoding/json"
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	"io"
	"net/http"
)

const (
	assetListUrl = "https://raw.githubusercontent.com/faneaatiku/chain-registry/refs/heads/master/beezee/assetlist.json"
)

type ChainRegistry struct {
}

func NewChainRegistry() (*ChainRegistry, error) {
	return &ChainRegistry{}, nil
}

func (r ChainRegistry) GetAssetList() (*chain_registry.ChainRegistryAssetList, error) {
	resp, err := http.Get(assetListUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JSON: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch JSON: received status %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Decode the JSON into the AssetList structure
	var assetList chain_registry.ChainRegistryAssetList
	err = json.Unmarshal(body, &assetList)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	return &assetList, nil
}
