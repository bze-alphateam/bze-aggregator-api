package calculator

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CalculatePriceChange calculates the percentage change from the opening price to the last price.
// Returns the percentage change as a float64 rounded to two decimal places.
func CalculatePriceChange(openingPrice, lastPrice sdk.Dec) sdk.Dec {
	if !openingPrice.IsPositive() {
		return sdk.ZeroDec() // Avoid division by zero
	}

	//change := ((lastPrice - openingPrice) / openingPrice) * 100
	change := lastPrice.Sub(openingPrice).Quo(openingPrice).MulInt64(100)

	return change
}
