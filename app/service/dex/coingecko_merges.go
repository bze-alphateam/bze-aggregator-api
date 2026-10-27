package dex

import (
	"fmt"
	"slices"
)

// CoingeckoMerges folds markets into a market CoinGecko already tracks, for the
// coingecko format only. When a market's quote asset is replaced (USDC.n by
// USDC.inj), the new market's figures are reported under the old market's
// ticker id, so CoinGecko keeps reading the ticker it knows. Keys are the
// folded (new) market ids, values the reported (old) ones. The default format
// and the data collection keep every market separate.
type CoingeckoMerges map[string]string

// NewCoingeckoMerges validates folded → reported market ids: a market is not
// folded into itself, and a reported market is not folded into another one.
// An empty or nil map merges nothing.
func NewCoingeckoMerges(folded map[string]string) (CoingeckoMerges, error) {
	merges := make(CoingeckoMerges, len(folded))
	for from, into := range folded {
		if from == into {
			return nil, fmt.Errorf("market %s cannot be folded into itself", from)
		}

		if _, ok := folded[into]; ok {
			return nil, fmt.Errorf("market %s is folded into %s, which is folded itself", from, into)
		}

		merges[from] = into
	}

	return merges, nil
}

// foldedInto returns the markets folded into reported, sorted.
func (m CoingeckoMerges) foldedInto(reported string) []string {
	var folded []string
	for from, into := range m {
		if into == reported {
			folded = append(folded, from)
		}
	}

	slices.Sort(folded)

	return folded
}
