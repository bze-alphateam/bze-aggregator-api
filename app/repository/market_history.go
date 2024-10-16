package repository

import "github.com/bze-alphateam/bze-aggregator-api/internal"

type MarketHistoryRepository struct {
	db internal.Database
}

func NewMarketHistoryRepository(db internal.Database) (*MarketHistoryRepository, error) {
	if db == nil {
		return nil, internal.NewInvalidDependenciesErr("MarketHistoryRepository")
	}

	return &MarketHistoryRepository{db: db}, nil
}
