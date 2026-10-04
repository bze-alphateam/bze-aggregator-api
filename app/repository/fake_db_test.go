package repository

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

var errFakeQuery = errors.New("fake query error")

// fakeDB records the args of the last Select/Queryx call. Select succeeds
// without touching dest, Queryx fails so callers stop before reading rows.
type fakeDB struct {
	args []interface{}
}

func (f *fakeDB) NamedExec(string, interface{}) (sql.Result, error) { return nil, nil }
func (f *fakeDB) Beginx() (*sqlx.Tx, error)                         { return nil, nil }
func (f *fakeDB) Get(interface{}, string, ...interface{}) error     { return nil }
func (f *fakeDB) Exec(string, ...any) (sql.Result, error)           { return nil, nil }

func (f *fakeDB) Select(_ interface{}, _ string, args ...interface{}) error {
	f.args = args
	return nil
}

func (f *fakeDB) Queryx(_ string, args ...interface{}) (*sqlx.Rows, error) {
	f.args = args
	return nil, errFakeQuery
}

// timeWithNanos has a non-zero sub-microsecond component, like time.Now().
var timeWithNanos = time.Date(2026, 10, 3, 15, 42, 34, 81401330, time.UTC)

func boundTime(t *testing.T, args []interface{}, idx int) time.Time {
	t.Helper()
	if len(args) <= idx {
		t.Fatalf("expected at least %d bound args, got %d", idx+1, len(args))
	}
	tm, ok := args[idx].(time.Time)
	if !ok {
		t.Fatalf("arg %d: expected time.Time, got %T", idx, args[idx])
	}

	return tm
}

func assertNoSubMicros(t *testing.T, tm time.Time) {
	t.Helper()
	if tm.Nanosecond()%1000 != 0 {
		t.Fatalf("bound time %s has a sub-microsecond component (%dns)", tm.Format(time.RFC3339Nano), tm.Nanosecond()%1000)
	}
}
