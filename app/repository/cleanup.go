package repository

import (
	"fmt"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/internal"
)

type CleanupRepository struct {
	db internal.Database
}

func NewCleanupRepository(db internal.Database) (*CleanupRepository, error) {
	if db == nil {
		return nil, internal.NewInvalidDependenciesErr("NewCleanupRepository")
	}

	return &CleanupRepository{db: db}, nil
}

func (r *CleanupRepository) DeleteOldBlocks(days int) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -days)

	// Delete blocks that don't have events with type starting with "bze."
	// CASCADE will automatically delete related data
	deleteBlocksQuery := `
		DELETE FROM blocks
		WHERE created_at < $1
		AND NOT EXISTS (
			SELECT 1 FROM events
			WHERE events.block_id = blocks.rowid
			AND events.type LIKE 'bze.%'
		)
	`
	result, err := r.db.Exec(deleteBlocksQuery, cutoffDate)
	if err != nil {
		return 0, fmt.Errorf("error deleting blocks: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error getting rows affected: %w", err)
	}

	return rowsAffected, nil
}
