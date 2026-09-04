package entity

import (
	"database/sql"
	"time"
)

// BackfillCheckpoint is the single row of temp_checkpoint. It records the
// bounds of a swap back-fill scan and how far it got. The scan walks
// descending, so NextHeight decreases as the scan progresses.
type BackfillCheckpoint struct {
	ID           int       `db:"id"`
	InitHeight   int64     `db:"init_height"`
	FloorHeight  int64     `db:"floor_height"`
	NextHeight   int64     `db:"next_height"`
	ScanComplete bool      `db:"scan_complete"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// StagedSwap is a swap recovered from an archive node and parked in
// temp_market_history. It mirrors the market_history insert columns and adds
// provenance (which block/tx/event it came from) plus the commit flag that
// makes a crashed commit resumable without duplicates.
type StagedSwap struct {
	ID          int       `db:"id"`
	MarketID    string    `db:"market_id"`
	OrderType   string    `db:"order_type"`
	Amount      string    `db:"amount"`
	Price       string    `db:"price"`
	ExecutedAt  time.Time `db:"executed_at"`
	Maker       string    `db:"maker"`
	Taker       string    `db:"taker"`
	QuoteAmount string    `db:"i_quote_amount"`
	Height      int64     `db:"height"`
	TxIndex     int       `db:"tx_index"`
	EventIndex  int       `db:"event_index"`
	Committed   bool      `db:"committed"`
}

// ToMarketHistory converts a staged row into the live market_history entity.
// addedToInterval decides whether the row's candles are written by the
// back-fill (true) or left for the listener to build (false).
func (s *StagedSwap) ToMarketHistory(addedToInterval bool) *MarketHistory {
	return &MarketHistory{
		MarketID:        s.MarketID,
		OrderType:       s.OrderType,
		Amount:          s.Amount,
		Price:           s.Price,
		ExecutedAt:      s.ExecutedAt,
		Maker:           s.Maker,
		Taker:           s.Taker,
		QuoteAmount:     s.QuoteAmount,
		AddedToInterval: addedToInterval,
	}
}

// StagedCounts is the row count summary of temp_market_history.
type StagedCounts struct {
	Total     int64 `db:"total"`
	Committed int64 `db:"committed"`
}

func (c StagedCounts) Uncommitted() int64 {
	return c.Total - c.Committed
}

// StagedStats aggregates a slice of staged rows (used for the pre-commit report).
type StagedStats struct {
	Count         int64         `db:"total_count"`
	MinExecutedAt sql.NullTime  `db:"min_executed_at"`
	MaxExecutedAt sql.NullTime  `db:"max_executed_at"`
	MinHeight     sql.NullInt64 `db:"min_height"`
	MaxHeight     sql.NullInt64 `db:"max_height"`
}

// PoolOldestSwap is the oldest swap the aggregator still holds for a liquidity
// pool. A null timestamp means the pool has no history at all.
type PoolOldestSwap struct {
	MarketID   string       `db:"market_id"`
	OldestSwap sql.NullTime `db:"oldest_swap"`
}
