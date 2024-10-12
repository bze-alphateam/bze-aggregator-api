package internal

import "database/sql"

type Database interface {
	NamedExec(query string, arg interface{}) (sql.Result, error)
}
