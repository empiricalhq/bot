package database

import (
	"context"
	"database/sql"
	"time"

	_ "modernc.org/sqlite"

	"whatsbot/internal/logger"
)

const (
	maxOpenConns    = 10
	maxIdleConns    = 5
	connMaxLifetime = time.Hour
	pingTimeout     = 15 * time.Second
)

func NewSQLite(ctx context.Context, dbPath string, logger logger.Logger) (*sql.DB, error) {
	dsn := dbPath + "?_pragma=journal_mode=WAL&_pragma=busy_timeout=5000&_pragma=foreign_keys=ON"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		db.Close()

		return nil, err
	}

	logger.Info("Database connected", "path", dbPath)

	return db, nil
}
