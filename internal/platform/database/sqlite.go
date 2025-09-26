package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"whatsbot/internal/logger"
)

const (
	dbPingTimeout  = 15 * time.Second
	dbMaxOpenConns = 10
	dbMaxIdleConns = 5
)

// NewSQLite initializes and returns a new SQLite database connection pool.
func NewSQLite(ctx context.Context, dbPath string, log *logger.Logger) (*sql.DB, error) {
	// WAL mode is highly recommended for SQLite to improve concurrency.
	dsn := dbPath + "?_pragma=journal_mode=WAL&_pragma=busy_timeout=5000&_pragma=foreign_keys=ON"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	db.SetMaxOpenConns(dbMaxOpenConns)
	db.SetMaxIdleConns(dbMaxIdleConns)
	db.SetConnMaxLifetime(time.Hour)

	pingCtx, cancel := context.WithTimeout(ctx, dbPingTimeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		db.Close() // Best effort to clean up.

		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Database connection pool initialized", map[string]interface{}{"path": dbPath})

	return db, nil
}
