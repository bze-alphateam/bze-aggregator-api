package converter

import (
	"fmt"
	"math"

	math2 "cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
)

const (
	defaultMaxDecimals = 6
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
