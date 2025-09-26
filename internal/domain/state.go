package domain

import "time"

// UserState represents the complete state of a user's conversation.
type UserState struct {
	UserID             string
	CurrentNode        string
	UserName           string
	CourseInterest     string
	ConsultedPrice     bool
	RequiresHumanAgent bool
	LastUpdated        time.Time
}

// ConversationMessage represents a single message in the conversation history.
type ConversationMessage struct {
	UserID         string
	Timestamp      time.Time
	Direction      string
	MessageContent string
	NodeID         string
}
