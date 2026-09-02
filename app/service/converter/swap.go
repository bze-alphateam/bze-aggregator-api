package converter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/cosmos/cosmos-sdk/types"
)

func ConvertEventToSwapData(event *entity.Event, attributes []entity.EventAttribute) (*dto.SwapEventData, error) {
	data := &dto.SwapEventData{
		EventID:    event.RowID,
		ExecutedAt: event.CreatedAt,
	}

	for _, attr := range attributes {
		switch attr.Key {
		case "pool_id":
			// Remove surrounding quotes (e.g., """value""" -> value)
			data.PoolID = strings.Trim(attr.Value, `"`)
		case "creator":
			// Remove surrounding quotes
			data.Creator = strings.Trim(attr.Value, `"`)
		case "in":
			// Parse JSON: {"denom":"utbz","amount":"32000000"}
			var coin types.Coin
			err := json.Unmarshal([]byte(attr.Value), &coin)
			if err != nil {
				return nil, fmt.Errorf("error parsing 'in' attribute: %w", err)
			}
			data.Input = coin
		case "out":
			// Parse JSON: {"denom":"factory/.../vidulum","amount":"31678936"}
			var coin types.Coin
			err := json.Unmarshal([]byte(attr.Value), &coin)
			if err != nil {
				return nil, fmt.Errorf("error parsing 'out' attribute: %w", err)
			}
			data.Output = coin
		}
	}

	// Validate required fields
	if data.PoolID == "" || data.Creator == "" || !data.Input.IsValid() || !data.Output.IsValid() {
		return nil, fmt.Errorf("missing required swap event attributes")
	}

	return data, nil
}

func PoolIdToDenoms(poolId string) (base, quote string, err error) {
	// pool id format is <base_denom>_<quote_denom>
	parts := strings.Split(poolId, "_")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid pool id format")
	}

	return parts[0], parts[1], nil
}
