package dto

import "time"

type MarketHealth struct {
	IsHealthy bool      `json:"is_healthy"`
	LastTrade time.Time `json:"last_trade"`
}

type AggregatorHealth struct {
	IsHealthy bool      `json:"is_healthy"`
	LastSync  time.Time `json:"last_sync"`
}

type NodesHealth struct {
	IsHealthy bool   `json:"is_healthy"`
	Errors    string `json:"errors"`
}

type AddressHealthCheck struct {
	Address   string `json:"address"`
	IsHealthy bool   `json:"is_healthy"`
	Error     string `json:"error"`
}
