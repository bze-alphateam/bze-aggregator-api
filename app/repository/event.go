package repository

import (
	"fmt"

	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
)

type EventRepository struct {
	db internal.Database
}

func NewEventRepository(db internal.Database) (*EventRepository, error) {
	if db == nil {
		return nil, internal.NewInvalidDependenciesErr("NewEventRepository")
	}

	return &EventRepository{db: db}, nil
}

func (r *EventRepository) GetUnprocessedSwapEvents(limit int) ([]entity.Event, error) {
	query := `
		SELECT e.rowid, e.block_id, b.height, e.tx_id, e.type, e.status, b.created_at
		FROM events e
		JOIN blocks b ON e.block_id = b.rowid
		WHERE e.type = 'bze.tradebin.SwapEvent' AND e.status = 0
		ORDER BY e.rowid ASC
		LIMIT $1
	`

	var events []entity.Event
	err := r.db.Select(&events, query, limit)
	if err != nil {
		return nil, fmt.Errorf("error fetching unprocessed swap events: %w", err)
	}

	return events, nil
}

func (r *EventRepository) GetEventAttributes(eventID int64) ([]entity.EventAttribute, error) {
	query := `
		SELECT event_id, key, composite_key, value
		FROM attributes
		WHERE event_id = $1
	`

	var attributes []entity.EventAttribute
	err := r.db.Select(&attributes, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("error fetching event attributes for event %d: %w", eventID, err)
	}

	return attributes, nil
}

func (r *EventRepository) MarkEventAsProcessed(eventID int64) error {
	query := `
		UPDATE events
		SET status = 1
		WHERE rowid = $1
	`

	_, err := r.db.Exec(query, eventID)
	if err != nil {
		return fmt.Errorf("error marking event %d as processed: %w", eventID, err)
	}

	return nil
}
