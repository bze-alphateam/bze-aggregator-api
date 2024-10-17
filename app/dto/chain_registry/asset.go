package chain_registry

type ChainRegistryAssetDenom struct {
	Denom    string   `json:"denom"`
	Exponent int      `json:"exponent"`
	Aliases  []string `json:"aliases,omitempty"`
}

type ChainRegistryAsset struct {
	DenomUnits []ChainRegistryAssetDenom `json:"denom_units"`
	Base       string                    `json:"base"`
	Name       string                    `json:"name"`
	Display    string                    `json:"display"`
	Symbol     string                    `json:"symbol"`
}

// ChainRegistryAssetList -https://github.com/cosmos/chain-registry/blob/master/beezee/assetlist.json
type ChainRegistryAssetList struct {
	ChainName string               `json:"chain_name"`
	Assets    []ChainRegistryAsset `json:"assets"`
}

func (a *ChainRegistryAsset) GetDisplayDenomUnit() *ChainRegistryAssetDenom {
	for _, d := range a.DenomUnits {
		if d.Denom == a.Display {
			return &d
		}
	}

	return nil
}
