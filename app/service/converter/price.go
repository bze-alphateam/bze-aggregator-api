package converter

import (
	"fmt"
	"math"
	"strings"

	math2 "cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
)

func UPriceToPrice(base, quote *chain_registry.ChainRegistryAsset, price string) (string, float64, error) {
	bd := base.GetDisplayDenomUnit()
	if bd == nil {
		return "", 0, fmt.Errorf("no display denom for base asset")
	}

	qd := quote.GetDisplayDenomUnit()
	if qd == nil {
		return "", 0, fmt.Errorf("no display denom for quote asset")
	}

	priceDec, err := math2.LegacyNewDecFromStr(price)
	if err != nil {
		return "", 0, err
	}

	if bd.Exponent == qd.Exponent {
		return TrimAmountTrailingZeros(price), priceDec.MustFloat64(), nil
	}

	multiplier := math2.LegacyMustNewDecFromStr(fmt.Sprintf("%.2f", math.Pow10(bd.Exponent-qd.Exponent)))
	priceDec = priceDec.Mul(multiplier)

	return TrimAmountTrailingZeros(priceDec.String()), priceDec.MustFloat64(), nil
}

func UAmountToAmount(asset *chain_registry.ChainRegistryAsset, amount string) (string, error) {
	displayDenomUnit := asset.GetDisplayDenomUnit()
	if displayDenomUnit == nil {
		return "", fmt.Errorf("no display denom for asset")
	}

	amtInt, _ := math2.NewIntFromString(amount)
	decAmount := math2.LegacyNewDecWithPrec(amtInt.Int64(), int64(displayDenomUnit.Exponent))

	return TrimAmountTrailingZeros(decAmount.String()), nil
}

func TrimAmountTrailingZeros(amount string) string {
	if !strings.Contains(amount, ".") {
		return amount
	}

	result := strings.TrimRight(amount, "0") // Remove trailing zeros
	if strings.HasSuffix(result, ".") {
		result = strings.TrimSuffix(result, ".") // Remove decimal point if no fractional part
	}

	return result
}

func DecToFloat32Rounded(decimal math2.LegacyDec) float32 {
	// Convert sdk.Dec to float64, round to 2 decimals, and convert to float32
	rounded := math.Round(decimal.MustFloat64()*100) / 100
	return float32(rounded)
}
