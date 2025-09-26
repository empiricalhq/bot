package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

type StmtCache struct {
	mu    sync.RWMutex
	cache map[string]*sql.Stmt
}

// NewStmtCache creates and prepares all required statements.
func NewStmtCache(ctx context.Context, db *sql.DB) (*StmtCache, error) {
	c := &StmtCache{
		cache: make(map[string]*sql.Stmt),
	}

	queries := []string{
		queryGetUserState,
		querySaveUserState,
		querySaveMessage,
	}
	for _, q := range queries {
		stmt, err := db.PrepareContext(ctx, q)
		if err != nil {
			for _, s := range c.cache {
				s.Close()
			}

			return nil, fmt.Errorf("failed to prepare query '%s': %w", q, err)
		}

		c.cache[q] = stmt
	}

	return c, nil
}

// Get retrieves a prepared statement from the cache.
func (c *StmtCache) Get(query string) (*sql.Stmt, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stmt, ok := c.cache[query]
	if !ok {
		return nil, fmt.Errorf("statement not found in cache: %s", query)
	}

	return stmt, nil
}

// GetTx returns a transaction-specific statement from a global prepared statement.
func (c *StmtCache) GetTx(tx *sql.Tx, query string) (*sql.Stmt, error) {
	stmt, err := c.Get(query)
	if err != nil {
		return nil, err
	}

	return tx.StmtContext(context.Background(), stmt), nil
}
