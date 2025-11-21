package response

type BalanceHealthResponse struct {
	IsHealthy bool   `json:"is_healthy"`
	Errors    string `json:"errors"`
}
