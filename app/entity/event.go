package entity

import (
	"time"
)

// Event represents a blockchain event stored in PostgreSQL
type Event struct {
	RowID       int64     `db:"rowid"`
	BlockID     int64     `db:"block_id"`
	BlockHeight int64     `db:"height"`
	TxID        *int64    `db:"tx_id"`
	Type        string    `db:"type"`
	Status      int16     `db:"status"`
	CreatedAt   time.Time `db:"created_at"`
}

// EventAttribute represents an attribute of an event
type EventAttribute struct {
	EventID      int64  `db:"event_id"`
	Key          string `db:"key"`
	CompositeKey string `db:"composite_key"`
	Value        string `db:"value"`
}
