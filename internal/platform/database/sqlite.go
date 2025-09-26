package database

import (
	"context"
	"database/sql"
	"time"

	_ "modernc.org/sqlite"

	"whatsbot/internal/logger"
)

func NewSQLite(ctx context.Context, dbPath string, logger logger.Logger) (*sql.DB, error) {
	dsn := dbPath + "?_pragma=journal_mode=WAL&_pragma=busy_timeout=5000&_pragma=foreign_keys=ON"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		db.Close()

		return nil, err
	}

	logger.Info("Database connected", "path", dbPath)

	return db, nil
}
