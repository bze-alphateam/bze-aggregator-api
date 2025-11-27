package repository

import (
	"database/sql"
	"errors"
	"time"

	"cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
)

type MarketRepository struct {
	db internal.Database
}

func NewMarketRepository(db internal.Database) (*MarketRepository, error) {
	if db == nil {
		return nil, internal.NewInvalidDependenciesErr("NewMarketRepository")
	}

	return &MarketRepository{db: db}, nil
}

func (r *MarketRepository) GetMarket(marketId string) (*entity.Market, error) {
	query := `
		SELECT * FROM market WHERE market_id = ?;
	`

	ent := &entity.Market{}
	err := r.db.Get(ent, query, marketId)
	if err == nil {
		return ent, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	return nil, err
}

func (r *MarketRepository) SaveIfNotExists(items []*entity.Market) error {
	query := `
	INSERT INTO market (
		market_id, base, quote, created_by, i_created_at
	) VALUES (
		:market_id, :base, :quote, :created_by, :i_created_at
	) 
	ON DUPLICATE KEY UPDATE 
		i_created_at = VALUES(i_created_at);`

	_, err := r.db.NamedExec(query, items)
	if err != nil {
		return err
	}

	return nil
}

func (r *MarketRepository) GetMarketsWithLastExecuted(hours int) ([]entity.MarketWithLastPrice, error) {
	query := `
		SELECT 
		    m.id as id,
			m.market_id as market_id,
			m.base as base,
			m.quote as quote,
			m.created_by as created_by,
			m.i_created_at as i_created_at,
			mh.price as last_price
		FROM market m
		LEFT JOIN (
			SELECT 
				market_id,
				price,
				ROW_NUMBER() OVER (PARTITION BY market_id ORDER BY executed_at DESC) as rn
			FROM market_history
			WHERE executed_at > ?
		) mh ON mh.market_id = m.market_id AND mh.rn = 1
		ORDER BY m.id ASC;
`
	executedAt := time.Now().Add(-time.Hour * time.Duration(hours))
	var results []entity.MarketWithLastPrice
	err := r.db.Select(&results, query, executedAt)
	if err == nil {
		return r.groupDuplicateMarketsWithLastPrice(results), nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return results, nil
	}

	return nil, err
}

// groupDuplicateMarketsWithLastPrice processes a list of markets to group duplicates and calculate the average price for each market.
// It returns a deduplicated slice where each market has a last price averaged across all occurrences in the input slice.
func (r *MarketRepository) groupDuplicateMarketsWithLastPrice(items []entity.MarketWithLastPrice) []entity.MarketWithLastPrice {
	marketsSums := make(map[string]math.LegacyDec)
	marketsCount := make(map[string]int64)
	var duplicatesRemoved []entity.MarketWithLastPrice
	for _, i := range items {
		_, ok := marketsSums[i.MarketID]
		if !ok {
			duplicatesRemoved = append(duplicatesRemoved, i)
			marketsSums[i.MarketID] = math.LegacyZeroDec()
		}

		_, ok = marketsCount[i.MarketID]
		if !ok {
			marketsCount[i.MarketID] = 0
		}

		marketsCount[i.MarketID]++

		priceDec := math.LegacyZeroDec()
		if i.LastPrice.Valid {
			priceDec = math.LegacyMustNewDecFromStr(i.LastPrice.String)
		}

		marketsSums[i.MarketID] = marketsSums[i.MarketID].Add(priceDec)
	}

	for _, i := range duplicatesRemoved {
		total, ok := marketsSums[i.MarketID]
		if !ok {
			//should never happen
			continue
		}

		counter, ok := marketsCount[i.MarketID]
		if !ok {
			//should never happen
			continue
		}

		i.LastPrice = sql.NullString{
			String: total.QuoInt64(counter).String(),
			Valid:  true,
		}
	}

	return duplicatesRemoved
}
