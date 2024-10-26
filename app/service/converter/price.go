package converter

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"math"
	"strings"
)

func UPriceToPrice(base, quote *chain_registry.ChainRegistryAsset, price string) (string, error) {
	bd := base.GetDisplayDenomUnit()
	if bd == nil {
		return "", fmt.Errorf("no display denom for base asset")
	}

	qd := quote.GetDisplayDenomUnit()
	if qd == nil {
		return "", fmt.Errorf("no display denom for quote asset")
	}

	if bd.Exponent == qd.Exponent {
		return TrimAmountTrailingZeros(price), nil
	}

	priceDec, err := sdk.NewDecFromStr(price)
	if err != nil {
		return "", err
	}

	multiplier := sdk.MustNewDecFromStr(fmt.Sprintf("%.2f", math.Pow10(bd.Exponent-qd.Exponent)))
	priceDec = priceDec.Mul(multiplier)

	return TrimAmountTrailingZeros(priceDec.String()), nil
}

func UAmountToAmount(asset *chain_registry.ChainRegistryAsset, amount string) (string, error) {
	displayDenomUnit := asset.GetDisplayDenomUnit()
	if displayDenomUnit == nil {
		return "", fmt.Errorf("no display denom for asset")
	}

	amtInt, _ := sdk.NewIntFromString(amount)
	decAmount := sdk.NewDecWithPrec(amtInt.Int64(), int64(displayDenomUnit.Exponent))

	return TrimAmountTrailingZeros(decAmount.String()), nil
}

func TrimAmountTrailingZeros(amount string) string {
	result := strings.TrimRight(amount, "0") // Remove trailing zeros
	if strings.HasSuffix(result, ".") {
		result = strings.TrimSuffix(result, ".") // Remove decimal point if no fractional part
	}

	return result
}
