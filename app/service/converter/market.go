package converter

import (
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func GetMarketId(base, quote string) string {
	return fmt.Sprintf("%s/%s", base, quote)
}

func GetQuoteAmount(baseAmount uint64, price string) (uint64, error) {
	oAmount := sdk.NewDecFromInt(sdk.NewIntFromUint64(baseAmount))
	oPrice, err := sdk.NewDecFromStr(price)
	if err != nil {
		return 0, err
	}

	oAmount = oAmount.Mul(oPrice)
	oAmount = oAmount.TruncateDec()

	return oAmount.TruncateInt().Uint64(), nil
}
