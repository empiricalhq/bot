package repository

import (
	"context"
	"database/sql"
	"errors"

	"whatsbot/internal/domain"
	"whatsbot/internal/logger"
)

type Repository interface {
	GetUserState(ctx context.Context, userID string) (*domain.UserState, error)
	SaveStateAndMessages(ctx context.Context, state *domain.UserState, inMsg, outMsg *domain.ConversationMessage) error
	InitSchema(ctx context.Context) error
}

type repository struct {
	db     *sql.DB
	logger logger.Logger
}

func New(db *sql.DB, logger logger.Logger) Repository {
	return &repository{db: db, logger: logger}
}

const getUserStateQuery = `
	SELECT current_node, user_name, course_interest, consulted_price, requires_human_agent, last_updated
	FROM user_state WHERE user_id = ?
`

const saveUserStateQuery = `
	INSERT INTO user_state (user_id, current_node, user_name, course_interest, consulted_price, requires_human_agent, last_updated)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id) DO UPDATE SET
		current_node = excluded.current_node,
		user_name = excluded.user_name,
		course_interest = excluded.course_interest,
		consulted_price = excluded.consulted_price,
		requires_human_agent = excluded.requires_human_agent,
		last_updated = excluded.last_updated
`

const saveMessageQuery = `
	INSERT INTO conversation_history (user_id, timestamp, direction, message_content, node_id)
	VALUES (?, ?, ?, ?, ?)
`

func (r *repository) GetUserState(ctx context.Context, userID string) (*domain.UserState, error) {
	state := &domain.UserState{UserID: userID}

	err := r.db.QueryRowContext(ctx, getUserStateQuery, userID).Scan(
		&state.CurrentNode,
		&state.UserName,
		&state.CourseInterest,
		&state.ConsultedPrice,
		&state.RequiresHumanAgent,
		&state.LastUpdated,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return state, nil // Return empty state for new users
	}

	if err != nil {
		r.logger.Error("Failed to get user state", "error", err, "user", userID)

		return nil, err
	}

	return state, nil
}

func (r *repository) SaveStateAndMessages(
	ctx context.Context,
	state *domain.UserState,
	inMsg, outMsg *domain.ConversationMessage,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Save user state
	_, err = tx.ExecContext(ctx, saveUserStateQuery,
		state.UserID,
		state.CurrentNode,
		state.UserName,
		state.CourseInterest,
		state.ConsultedPrice,
		state.RequiresHumanAgent,
		state.LastUpdated,
	)
	if err != nil {
		return err
	}

	// Save inbound message
	if inMsg != nil {
		_, err = tx.ExecContext(ctx, saveMessageQuery,
			inMsg.UserID,
			inMsg.Timestamp,
			inMsg.Direction,
			inMsg.MessageContent,
			inMsg.NodeID,
		)
		if err != nil {
			return err
		}
	}

	// Save outbound message
	if outMsg != nil {
		_, err = tx.ExecContext(ctx, saveMessageQuery,
			outMsg.UserID,
			outMsg.Timestamp,
			outMsg.Direction,
			outMsg.MessageContent,
			outMsg.NodeID,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *repository) InitSchema(ctx context.Context) error {
	queries := []string{
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

	for _, query := range queries {
		_, err := r.db.ExecContext(ctx, query)
		if err != nil {
			return err
		}
	}

	r.logger.Info("Database schema initialized")

	return nil
}
