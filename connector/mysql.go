package connector

import (
	"github.com/jmoiron/sqlx"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func NewDatabaseConnection() (*sqlx.DB, error) {
	db, err := sqlx.Connect("mysql", os.Getenv("MYSQL_DSN"))
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(50)
	db.SetConnMaxLifetime(time.Second * 5)
	db.SetConnMaxIdleTime(time.Second * 5)

	if err = db.Ping(); nil != err {
		return nil, err
	}

	return db, nil
}
