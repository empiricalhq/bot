package domain

import "time"

type UserState struct {
	UserID             string    `db:"user_id"`
	CurrentNode        string    `db:"current_node"`
	UserName           string    `db:"user_name"`
	CourseInterest     string    `db:"course_interest"`
	ConsultedPrice     bool      `db:"consulted_price"`
	RequiresHumanAgent bool      `db:"requires_human_agent"`
	LastUpdated        time.Time `db:"last_updated"`
}

type ConversationMessage struct {
	UserID         string    `db:"user_id"`
	Timestamp      time.Time `db:"timestamp"`
	Direction      string    `db:"direction"`
	MessageContent string    `db:"message_content"`
	NodeID         string    `db:"node_id"`
}
