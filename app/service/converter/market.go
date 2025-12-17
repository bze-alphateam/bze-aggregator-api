package converter

import (
	"fmt"
	"math"
	"strings"

	math2 "cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
)

const (
	defaultMaxDecimals = 6
	lpDenomPrefix      = "ulp_"
)

func GetMarketId(base, quote string) string {
	return fmt.Sprintf("%s/%s", base, quote)
}

func GetQuoteAmount(baseAmount string, price string, quoteAsset *chain_registry.ChainRegistryAsset) string {
	maxDecimals := defaultMaxDecimals
	bd := quoteAsset.GetDisplayDenomUnit()
	if bd != nil {
		maxDecimals = bd.Exponent
	}

	//multiply by scale -> truncate int -> divide by scale in order to keep max decimals
	scale := math2.LegacyNewDec(int64(math.Pow10(maxDecimals)))
	amt := math2.LegacyMustNewDecFromStr(baseAmount)
	p := math2.LegacyMustNewDecFromStr(price)

	total := amt.Mul(p).Mul(scale).TruncateDec().Quo(scale)

	return TrimAmountTrailingZeros(total.String())
}

// CreatePoolId creates a pool ID from base and quote denoms (underscore separator, lexicographically sorted)
func CreatePoolId(base, quote string) string {
	if base > quote {
		return fmt.Sprintf("%s_%s", quote, base)
	}
	return fmt.Sprintf("%s_%s", base, quote)
}

// IsLpDenom checks if a denom is an LP token
func IsLpDenom(denom string) bool {
	return strings.HasPrefix(denom, lpDenomPrefix)
}

// PoolIdFromPoolDenom extracts the pool ID from an LP denom
func PoolIdFromPoolDenom(poolDenom string) string {
	return strings.TrimPrefix(poolDenom, lpDenomPrefix)
}

func GetLpAssetDecimals() int {
	return 12
}
