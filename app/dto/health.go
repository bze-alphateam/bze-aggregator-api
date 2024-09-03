package dto

import "time"

type MarketHealth struct {
	IsHealthy bool      `json:"is_healthy"`
	LastTrade time.Time `json:"last_trade"`
}
