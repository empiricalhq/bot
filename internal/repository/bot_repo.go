package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"whatsbot/internal/domain"
	"whatsbot/internal/logger"
)

type BotRepository interface {
	GetUserState(ctx context.Context, userID string) (*domain.UserState, error)
	UpdateUserStateAndLog(ctx context.Context, state *domain.UserState, inMsg, outMsg *domain.ConversationMessage) error
	InitSchema(ctx context.Context) error
}

type botRepository struct {
	db     *sql.DB
	cache  *StmtCache
	logger *logger.Logger
}

func NewBotRepository(ctx context.Context, db *sql.DB, log *logger.Logger) (BotRepository, error) {
	cache, err := NewStmtCache(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to create prepared statement cache: %w", err)
	}

	return &botRepository{
		db:     db,
		cache:  cache,
		logger: log,
	}, nil
}

// GetUserState retrieves the state for a given user. If the user does not exist,
// it returns a new, zero-value UserState without an error.
func (r *botRepository) GetUserState(ctx context.Context, userID string) (*domain.UserState, error) {
	stmt, err := r.cache.Get(queryGetUserState)
	if err != nil {
		return nil, fmt.Errorf("failed to get prepared statement for GetUserState: %w", err)
	}

	userState := &domain.UserState{UserID: userID}

	err = stmt.QueryRowContext(ctx, userID).Scan(
		&userState.CurrentNode, &userState.UserName, &userState.CourseInterest,
		&userState.ConsultedPrice, &userState.RequiresHumanAgent, &userState.LastUpdated,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return userState, nil // new user: return empty state.
		}

		r.logger.Error("GetUserState query failed", map[string]interface{}{"error": err, "userID": userID})

		return nil, fmt.Errorf("failed to query user state for %s: %w", userID, err)
	}

	return userState, nil
}

func (r *botRepository) UpdateUserStateAndLog(
	ctx context.Context,
	userState *domain.UserState,
	inMsg, outMsg *domain.ConversationMessage,
) error {
	userState.LastUpdated = time.Now()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. save user state
	stmtState, err := r.cache.GetTx(tx, querySaveUserState)
	if err != nil {
		return err
	}

	_, err = stmtState.ExecContext(ctx,
		userState.UserID, userState.CurrentNode, userState.UserName, userState.CourseInterest,
		userState.ConsultedPrice, userState.RequiresHumanAgent, userState.LastUpdated,
	)
	if err != nil {
		return fmt.Errorf("failed to execute SaveUserState for %s: %w", userState.UserID, err)
	}

	// 2. save messages
	stmtMsg, err := r.cache.GetTx(tx, querySaveMessage)
	if err != nil {
		return err
	}

	if inMsg != nil {
		_, err = stmtMsg.ExecContext(ctx, inMsg.UserID, inMsg.Timestamp, inMsg.Direction, inMsg.MessageContent, inMsg.NodeID)
		if err != nil {
			return fmt.Errorf("failed to save inbound message: %w", err)
		}
	}

	if outMsg != nil {
		_, err = stmtMsg.ExecContext(ctx, outMsg.UserID, outMsg.Timestamp, outMsg.Direction, outMsg.MessageContent, outMsg.NodeID)
		if err != nil {
			return fmt.Errorf("failed to save outbound message: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		r.logger.Error("Transaction commit failed", map[string]interface{}{"error": err})

		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *botRepository) InitSchema(ctx context.Context) error {
	schemaQueries := []string{
		`CREATE TABLE IF NOT EXISTS user_state (
			user_id TEXT PRIMARY KEY,
			current_node TEXT NOT NULL DEFAULT '',
			user_name TEXT NOT NULL DEFAULT '',
			course_interest TEXT NOT NULL DEFAULT '',
			consulted_price BOOLEAN NOT NULL DEFAULT FALSE,
			requires_human_agent BOOLEAN NOT NULL DEFAULT FALSE,
			last_updated DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_state_updated ON user_state(last_updated)`,
		`CREATE TABLE IF NOT EXISTS conversation_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			direction TEXT NOT NULL CHECK(direction IN ('inbound', 'outbound')),
			message_content TEXT,
			node_id TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_conversation_user_time ON conversation_history(user_id, timestamp DESC)`,
	}

	for _, query := range schemaQueries {
		if _, err := r.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	r.logger.Info("Database schema initialized successfully", nil)

	return nil
}
