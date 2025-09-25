package state

import "time"

type UserState struct {
	UserID             string
	CurrentNode        string
	UserName           string
	CourseInterest     string
	ConsultedPrice     bool
	RequiresHumanAgent bool
	LastUpdated        time.Time
}

type ConversationMessage struct {
	UserID         string
	Timestamp      time.Time
	Direction      string
	MessageContent string
	NodeID         string
}
