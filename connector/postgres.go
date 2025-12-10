package connector

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"

	_ "github.com/lib/pq"
)

func NewPostgresConnection() (*sqlx.DB, error) {
	envFile, err := godotenv.Read(".env")
	if err != nil {
		return nil, err
	}

	dsn, ok := envFile["POSTGRES_DSN"]
	if !ok {
		return nil, fmt.Errorf("POSTGRES_DSN not found in .env")
	}

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Minute * 5)
	db.SetConnMaxIdleTime(time.Minute * 5)

	if err = db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
