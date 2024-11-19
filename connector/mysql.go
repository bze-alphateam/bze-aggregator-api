package connector

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func NewDatabaseConnection() (*sqlx.DB, error) {
	envFile, err := godotenv.Read(".env")
	if err != nil {
		return nil, err
	}

	dsn, ok := envFile["MYSQL_DSN"]
	if !ok {
		return nil, fmt.Errorf("MYSQL_DSN not found in .env")
	}

	db, err := sqlx.Connect("mysql", dsn)
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
