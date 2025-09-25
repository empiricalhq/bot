package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"whatsbot/internal/logger"
)

// Manager defines an interface for managing user state and conversation history.
type Manager interface {
	GetUserState(ctx context.Context, userID string) (*UserState, error)
	SaveUserState(ctx context.Context, state *UserState) error
	SaveMessage(ctx context.Context, msg *ConversationMessage) error
}

type SQLiteManager struct {
	db     *sql.DB
	logger *logger.Logger
}

func NewSQLiteManager(db *sql.DB, log *logger.Logger) (*SQLiteManager, error) {
	manager := &SQLiteManager{
		db:     db,
		logger: log,
	}

	err := manager.initSchema(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return manager, nil
}

func (m *SQLiteManager) GetUserState(ctx context.Context, userID string) (*UserState, error) {
	query := `SELECT current_node, user_name, course_interest, consulted_price, requires_human_agent, last_updated
			  FROM user_state WHERE user_id = ?`

	var userState UserState

	userState.UserID = userID

	err := m.db.QueryRowContext(ctx, query, userID).Scan(
		&userState.CurrentNode, &userState.UserName, &userState.CourseInterest, &userState.ConsultedPrice,
		&userState.RequiresHumanAgent, &userState.LastUpdated,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// for a new user: return empty state
			return &UserState{UserID: userID}, nil
		}

		return nil, fmt.Errorf("failed to get user state for %s: %w", userID, err)
	}

	return &userState, nil
}

func (m *SQLiteManager) SaveUserState(ctx context.Context, userState *UserState) error {
	userState.LastUpdated = time.Now()

	query := `
	INSERT INTO user_state (user_id, current_node, user_name, course_interest, consulted_price, requires_human_agent, last_updated)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id) DO UPDATE SET
		current_node = excluded.current_node,
		user_name = excluded.user_name,
		course_interest = excluded.course_interest,
		consulted_price = excluded.consulted_price,
		requires_human_agent = excluded.requires_human_agent,
		last_updated = excluded.last_updated`

	_, err := m.db.ExecContext(ctx, query,
		userState.UserID, userState.CurrentNode, userState.UserName, userState.CourseInterest,
		userState.ConsultedPrice, userState.RequiresHumanAgent, userState.LastUpdated,
	)
	if err != nil {
		return fmt.Errorf("failed to save user state for %s: %w", userState.UserID, err)
	}

	m.logger.Debug("User state saved", map[string]interface{}{
		"userID":      userState.UserID,
		"currentNode": userState.CurrentNode,
	})

	return nil
}

func (m *SQLiteManager) SaveMessage(ctx context.Context, msg *ConversationMessage) error {
	query := `
	INSERT INTO conversation_history (user_id, timestamp, direction, message_content, node_id)
	VALUES (?, ?, ?, ?, ?)`

	_, err := m.db.ExecContext(ctx, query,
		msg.UserID, msg.Timestamp, msg.Direction, msg.MessageContent, msg.NodeID,
	)
	if err != nil {
		return fmt.Errorf("failed to save message for %s: %w", msg.UserID, err)
	}

	return nil
}

// creates the necessary tables if they do not already exist.
func (m *SQLiteManager) initSchema(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS user_state (
			user_id TEXT PRIMARY KEY,
			current_node TEXT,
			user_name TEXT,
			course_interest TEXT,
			consulted_price BOOLEAN DEFAULT FALSE,
			requires_human_agent BOOLEAN DEFAULT FALSE,
			last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_state_updated ON user_state(last_updated)`,
		`CREATE TABLE IF NOT EXISTS conversation_history (
			user_id TEXT,
			timestamp DATETIME,
			direction TEXT CHECK(direction IN ('inbound', 'outbound')),
			message_content TEXT,
			node_id TEXT,
			PRIMARY KEY (user_id, timestamp)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_conversation_user_time ON conversation_history(user_id, timestamp DESC)`,
	}

	for _, query := range queries {
		_, err := m.db.ExecContext(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	return nil
}
