package internal

import (
	"database/sql"
	"github.com/jmoiron/sqlx"
)

type Database interface {
	NamedExec(query string, arg interface{}) (sql.Result, error)
	Beginx() (*sqlx.Tx, error)
}
