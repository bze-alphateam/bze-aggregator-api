package converter

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"math"
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
	scale := sdk.NewDec(int64(math.Pow10(maxDecimals)))
	amt := sdk.MustNewDecFromStr(baseAmount)
	p := sdk.MustNewDecFromStr(price)

	total := amt.Mul(p).Mul(scale).TruncateDec().Quo(scale)

	return TrimAmountTrailingZeros(total.String())
}
