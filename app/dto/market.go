package dto

import (
	"encoding/json"
	"strconv"
	"time"
)

type HistoryOrder struct {
	MarketId   string    `json:"market_id"`
	ExecutedAt time.Time `json:"executed_at"`
}

func (h *HistoryOrder) UnmarshalJSON(data []byte) error {
	// Define a temporary structure to unmarshal into
	var temp struct {
		MarketId   string `json:"market_id"`
		ExecutedAt string `json:"executed_at"` // ExecutedAt is a timestamp string here
	}

	// Unmarshal JSON data into the temporary structure
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Convert the ExecutedAt timestamp string to an integer
	timestamp, err := strconv.ParseInt(temp.ExecutedAt, 10, 64)
	if err != nil {
		return err
	}

	// Convert the Unix timestamp to time.Time
	h.ExecutedAt = time.Unix(timestamp, 0).UTC()

	// Assign the other value
	h.MarketId = temp.MarketId

	return nil
}

type HistoryResponse struct {
	List []HistoryOrder `json:"list"`
}
