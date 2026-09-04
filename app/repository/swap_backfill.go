package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/jmoiron/sqlx"
)

const (
	// CheckpointTable and StagedTable are created by `swap-backfill init` and
	// only by it. They are real tables (not TEMPORARY) on purpose: they have to
	// survive container restarts and crashes - that is the whole point of
	// staging the scan instead of writing market_history directly.
	CheckpointTable = "temp_checkpoint"
	StagedTable     = "temp_market_history"

	checkpointID = 1

	// insert statements are chunked so a day with a lot of swaps does not
	// produce one enormous statement. All chunks of a commit still run inside
	// the same transaction.
	insertBatchSize = 1000
)

var createCheckpointTable = fmt.Sprintf(`
CREATE TABLE %s (
    id INT NOT NULL,
    init_height BIGINT NOT NULL,
    floor_height BIGINT NOT NULL,
    next_height BIGINT NOT NULL,
    scan_complete TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    PRIMARY KEY (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;`, CheckpointTable)

var createStagedTable = fmt.Sprintf(`
CREATE TABLE %s (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    market_id VARCHAR(256) NOT NULL,
    order_type VARCHAR(16) NOT NULL,
    amount VARCHAR(80) NOT NULL,
    price VARCHAR(80) NOT NULL,
    executed_at DATETIME(3) NOT NULL,
    maker VARCHAR(64) NOT NULL,
    taker VARCHAR(64) NOT NULL,
    i_quote_amount VARCHAR(80) NOT NULL,
    height BIGINT NOT NULL,
    tx_index INT NOT NULL,
    event_index INT NOT NULL,
    committed TINYINT(1) NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    KEY idx_tmh_committed_executed (committed, executed_at),
    KEY idx_tmh_market_executed (market_id, executed_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;`, StagedTable)

var insertStagedQuery = fmt.Sprintf(`
	INSERT INTO %s (
		market_id, order_type, amount, price, executed_at, maker, taker, i_quote_amount,
		height, tx_index, event_index, committed
	) VALUES (
		:market_id, :order_type, :amount, :price, :executed_at, :maker, :taker, :i_quote_amount,
		:height, :tx_index, :event_index, 0
	);`, StagedTable)

// insertHistoryQuery mirrors MarketHistoryRepository.SaveMarketHistory. It is
// duplicated here because the insert has to share a transaction with the
// commit flag update - that atomicity is what guarantees a crashed commit
// never duplicates rows - and i_added_to_interval is set explicitly.
const insertHistoryQuery = `
	INSERT INTO market_history (
		market_id, order_type, amount, price, executed_at, maker, taker, i_quote_amount, i_created_at, i_added_to_interval
	) VALUES (
		:market_id, :order_type, :amount, :price, :executed_at, :maker, :taker, :i_quote_amount, NOW(), :i_added_to_interval
	);`

// upsertIntervalQuery mirrors MarketIntervalRepository.Save, for the same
// reason as insertHistoryQuery.
const upsertIntervalQuery = `
	INSERT INTO market_history_interval (
		market_id, length, start_at, end_at,
		lowest_price, open_price, average_price, highest_price, close_price,
		base_volume, quote_volume, i_created_at
	) VALUES (
		:market_id, :length, :start_at, :end_at,
		:lowest_price, :open_price, :average_price, :highest_price, :close_price,
		:base_volume, :quote_volume, NOW()
	)
	ON DUPLICATE KEY UPDATE
		lowest_price = VALUES(lowest_price),
		open_price = VALUES(open_price),
		average_price = VALUES(average_price),
		highest_price = VALUES(highest_price),
		close_price = VALUES(close_price),
		base_volume = VALUES(base_volume),
		quote_volume = VALUES(quote_volume),
		i_updated_at = NOW()
	;`

type SwapBackfillRepository struct {
	db internal.Database
}

func NewSwapBackfillRepository(db internal.Database) (*SwapBackfillRepository, error) {
	if db == nil {
		return nil, internal.NewInvalidDependenciesErr("NewSwapBackfillRepository")
	}

	return &SwapBackfillRepository{db: db}, nil
}

// TablesExist reports whether the staging tables are present. Both are created
// together, so anything other than "none" or "both" means someone has been
// editing the schema by hand.
func (r *SwapBackfillRepository) TablesExist() (bool, error) {
	query := `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name IN (?, ?)`

	var count int
	err := r.db.Get(&count, query, CheckpointTable, StagedTable)
	if err != nil {
		return false, err
	}

	if count == 1 {
		return false, fmt.Errorf("only one of the %s / %s tables exists - fix the schema by hand before continuing", CheckpointTable, StagedTable)
	}

	return count == 2, nil
}

func (r *SwapBackfillRepository) CreateTables() error {
	if _, err := r.db.Exec(createCheckpointTable); err != nil {
		return err
	}

	if _, err := r.db.Exec(createStagedTable); err != nil {
		return err
	}

	return nil
}

func (r *SwapBackfillRepository) DropTables() error {
	if _, err := r.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s;", StagedTable)); err != nil {
		return err
	}

	if _, err := r.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s;", CheckpointTable)); err != nil {
		return err
	}

	return nil
}

// RenameTables archives the staging tables instead of dropping them. They carry
// provenance and commit flags, so they are a permanent record of exactly what
// was inserted and where it came from.
func (r *SwapBackfillRepository) RenameTables(suffix string) (checkpointName, stagedName string, err error) {
	if err = validateTableSuffix(suffix); err != nil {
		return "", "", err
	}

	checkpointName = fmt.Sprintf("swap_backfill_checkpoint_%s", suffix)
	stagedName = fmt.Sprintf("swap_backfill_history_%s", suffix)

	query := fmt.Sprintf("RENAME TABLE %s TO %s, %s TO %s;", CheckpointTable, checkpointName, StagedTable, stagedName)
	if _, err = r.db.Exec(query); err != nil {
		return "", "", err
	}

	return checkpointName, stagedName, nil
}

func (r *SwapBackfillRepository) SaveCheckpoint(cp *entity.BackfillCheckpoint) error {
	query := fmt.Sprintf(`
	INSERT INTO %s (
		id, init_height, floor_height, next_height, scan_complete, created_at, updated_at
	) VALUES (
		:id, :init_height, :floor_height, :next_height, :scan_complete, :created_at, :updated_at
	);`, CheckpointTable)

	cp.ID = checkpointID
	_, err := r.db.NamedExec(query, cp)

	return err
}

func (r *SwapBackfillRepository) GetCheckpoint() (*entity.BackfillCheckpoint, error) {
	cp := entity.BackfillCheckpoint{}
	query := fmt.Sprintf("SELECT * FROM %s WHERE id = ?", CheckpointTable)

	err := r.db.Get(&cp, query, checkpointID)
	if err == nil {
		return &cp, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	return nil, err
}

func (r *SwapBackfillRepository) GetStagedCounts() (entity.StagedCounts, error) {
	counts := entity.StagedCounts{}
	query := fmt.Sprintf("SELECT COUNT(*) AS total, COALESCE(SUM(committed), 0) AS committed FROM %s", StagedTable)

	err := r.db.Get(&counts, query)

	return counts, err
}

// SaveScanChunk stores everything a scan chunk found and advances the
// checkpoint in one transaction, so an interrupted run resumes exactly where
// it stopped: either the rows and the new position are both there, or neither.
func (r *SwapBackfillRepository) SaveScanChunk(rows []*entity.StagedSwap, nextHeight int64, scanComplete bool) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, batch := range splitSlice(rows, insertBatchSize) {
		if _, err = tx.NamedExec(insertStagedQuery, batch); err != nil {
			return err
		}
	}

	query := fmt.Sprintf("UPDATE %s SET next_height = ?, scan_complete = ?, updated_at = NOW() WHERE id = ?", CheckpointTable)
	if _, err = tx.Exec(query, nextHeight, scanComplete, checkpointID); err != nil {
		return err
	}

	return tx.Commit()
}

// GetStagedMarketIds lists the markets present in the staging table. With
// uncommittedOnly it lists what is still left to commit; without it, every
// market the back-fill has touched.
func (r *SwapBackfillRepository) GetStagedMarketIds(uncommittedOnly bool) ([]string, error) {
	query := fmt.Sprintf("SELECT DISTINCT market_id FROM %s ORDER BY market_id", StagedTable)
	if uncommittedOnly {
		query = fmt.Sprintf("SELECT DISTINCT market_id FROM %s WHERE committed = 0 ORDER BY market_id", StagedTable)
	}

	var results []string
	err := r.db.Select(&results, query)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	return results, nil
}

// GetStagedStats aggregates the uncommitted rows of a market. from/to are
// optional executed_at bounds ([from, to)).
func (r *SwapBackfillRepository) GetStagedStats(marketId string, from, to *time.Time) (entity.StagedStats, error) {
	stats := entity.StagedStats{}
	query := fmt.Sprintf(`
		SELECT COUNT(*) AS total_count, MIN(executed_at) AS min_executed_at, MAX(executed_at) AS max_executed_at,
		       MIN(height) AS min_height, MAX(height) AS max_height
		FROM %s WHERE committed = 0 AND market_id = ?`, StagedTable)

	args := []interface{}{marketId}
	if from != nil {
		query += " AND executed_at >= ?"
		args = append(args, *from)
	}

	if to != nil {
		query += " AND executed_at < ?"
		args = append(args, *to)
	}

	err := r.db.Get(&stats, query, args...)

	return stats, err
}

// GetUncommittedBetween returns the uncommitted rows of one time window in
// chain order. Ordering by (height, tx_index, event_index) matters: all swaps
// of a block share the block header time, and the candle open/close comparisons
// break those ties by insertion order.
func (r *SwapBackfillRepository) GetUncommittedBetween(from, to time.Time) ([]entity.StagedSwap, error) {
	query := fmt.Sprintf(`
		SELECT * FROM %s
		WHERE committed = 0 AND executed_at >= ? AND executed_at < ?
		ORDER BY height ASC, tx_index ASC, event_index ASC`, StagedTable)

	var results []entity.StagedSwap
	err := r.db.Select(&results, query, from, to)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	return results, nil
}

// CommitChunk moves one window of staged rows into the live tables.
//
// Everything happens in a single transaction: the FOR UPDATE on the checkpoint
// row serialises against a second commit process, and inserting the history
// rows in the same transaction that flips their committed flag is what
// guarantees no duplicates - there is no state where the rows landed but the
// flag did not. Never run DDL in here: MySQL commits implicitly on
// CREATE/DROP/ALTER and that would silently break the guarantee.
func (r *SwapBackfillRepository) CommitChunk(rows []*entity.MarketHistory, intervals []*entity.MarketHistoryInterval, stagedIds []int) error {
	if len(rows) == 0 {
		return nil
	}

	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var lockedId int
	lockQuery := fmt.Sprintf("SELECT id FROM %s WHERE id = ? FOR UPDATE", CheckpointTable)
	if err = tx.Get(&lockedId, lockQuery, checkpointID); err != nil {
		return fmt.Errorf("could not lock the checkpoint row: %w", err)
	}

	for _, batch := range splitSlice(rows, insertBatchSize) {
		if _, err = tx.NamedExec(insertHistoryQuery, batch); err != nil {
			return err
		}
	}

	for _, batch := range splitSlice(intervals, insertBatchSize) {
		if _, err = tx.NamedExec(upsertIntervalQuery, batch); err != nil {
			return err
		}
	}

	updateQuery := fmt.Sprintf("UPDATE %s SET committed = 1 WHERE id IN (?)", StagedTable)
	for _, batch := range splitSlice(stagedIds, insertBatchSize) {
		query, args, inErr := sqlx.In(updateQuery, batch)
		if inErr != nil {
			return inErr
		}

		if _, err = tx.Exec(query, args...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetPoolsOldestSwap returns, for every known liquidity pool, the oldest swap
// still held in market_history. A NULL means the pool has no history at all.
// This is both the boundary the operator needs (`init`) and the overlap guard
// the commit uses: everything the listener already ingested is at or after it.
//
// excludeBackfilled leaves out the rows this back-fill has already committed,
// which is what the overlap guard has to ask. The guard means "the oldest swap
// the listener ingested", and a commit inserts rows older than that: read
// plainly on a resumed commit, market_history would report the back-fill's own
// first row as the boundary and every day it had not committed yet would look
// already-held and be dropped. Dating the markets wants the opposite - the
// oldest row that is actually there, back-filled or not.
func (r *SwapBackfillRepository) GetPoolsOldestSwap(excludeBackfilled bool) ([]entity.PoolOldestSwap, error) {
	// a committed staged row and a listener row can never share a timestamp:
	// the guard only ever lets us commit rows strictly older than the oldest
	// row the listener holds, so matching on (market_id, executed_at) only
	// takes out rows this back-fill inserted
	var backfilled string
	if excludeBackfilled {
		backfilled = fmt.Sprintf(`
			AND NOT EXISTS (
				SELECT 1 FROM %s t
				WHERE t.committed = 1
					AND t.market_id = mh.market_id
					AND t.executed_at = mh.executed_at
			)`, StagedTable)
	}

	query := fmt.Sprintf(`
		SELECT mld.market_id AS market_id, MIN(mh.executed_at) AS oldest_swap
		FROM market_liquidity_data mld
		LEFT JOIN market_history mh ON mh.market_id = mld.market_id%s
		GROUP BY mld.market_id
		ORDER BY mld.market_id`, backfilled)

	var results []entity.PoolOldestSwap
	err := r.db.Select(&results, query)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	return results, nil
}

func validateTableSuffix(suffix string) error {
	if len(suffix) == 0 || len(suffix) > 32 {
		return fmt.Errorf("invalid table suffix %q", suffix)
	}

	for _, c := range suffix {
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c == '_' {
			continue
		}

		return fmt.Errorf("invalid table suffix %q", suffix)
	}

	return nil
}

// splitSlice batches a slice so a single statement never gets unreasonably
// large. Batching does not split the transaction - the caller keeps all
// batches inside one.
func splitSlice[T any](data []T, batchSize int) [][]T {
	var batches [][]T
	for i := 0; i < len(data); i += batchSize {
		end := i + batchSize
		if end > len(data) {
			end = len(data)
		}
		batches = append(batches, data[i:end])
	}

	return batches
}
