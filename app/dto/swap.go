package dto

import (
	"time"

	"github.com/cosmos/cosmos-sdk/types"
)

// SwapEventData holds parsed swap event data
type SwapEventData struct {
	EventID    int64
	PoolID     string
	Creator    string
	Input      types.Coin
	Output     types.Coin
	ExecutedAt time.Time
}

func (e *SwapEventData) GetBase() types.Coin {
	if e.Input.Denom < e.Output.Denom {
		return e.Input
	}

	return e.Output
}

func (e *SwapEventData) GetQuote() types.Coin {
	if e.Input.Denom > e.Output.Denom {
		return e.Input
	}

	return e.Output
}
