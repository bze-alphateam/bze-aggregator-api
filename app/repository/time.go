package repository

import "time"

// sqlTime prepares a time.Time for binding as a MySQL query parameter.
//
// go-sql-driver sends time.Time with up to 9 fractional digits. MariaDB does
// not use an index range for a DATETIME literal with more than 6 fractional
// digits and falls back to a full scan, so every time derived from time.Now()
// must be truncated to microseconds before it is bound. Do not remove.
func sqlTime(t time.Time) time.Time {
	return t.Truncate(time.Microsecond)
}
