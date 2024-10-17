package converter

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"math"
)

func ConvertPrice(base, quote *chain_registry.ChainRegistryAsset, price string) (string, error) {
	bd := base.GetDisplayDenomUnit()
	if bd == nil {
		return "", fmt.Errorf("no display denom for base asset")
	}

	qd := quote.GetDisplayDenomUnit()
	if qd == nil {
		return "", fmt.Errorf("no display denom for quote asset")
	}

	if bd.Exponent == qd.Exponent {
		return price, nil
	}

	priceDec, err := sdk.NewDecFromStr(price)
	if err != nil {
		return "", err
	}

	multiplier := sdk.MustNewDecFromStr(fmt.Sprintf("%.2f", math.Pow10(bd.Exponent-qd.Exponent)))
	priceDec = priceDec.Mul(multiplier)

	return priceDec.String(), nil
}
