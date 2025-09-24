package state

import "time"

type UserState struct {
	UserID             string    `dynamodbav:"UserID"`
	CurrentNode        string    `dynamodbav:"CurrentNode"`
	UserName           string    `dynamodbav:"UserName,omitempty"`
	CourseInterest     string    `dynamodbav:"CourseInterest,omitempty"`
	ConsultedPrice     bool      `dynamodbav:"ConsultedPrice,omitempty"`
	RequiresHumanAgent bool      `dynamodbav:"RequiresHumanAgent,omitempty"`
	LastUpdated        time.Time `dynamodbav:"LastUpdated"`
}

type ConversationMessage struct {
	UserID         string    `dynamodbav:"UserID"`
	Timestamp      time.Time `dynamodbav:"Timestamp"`
	Direction      string    `dynamodbav:"Direction"`
	MessageContent string    `dynamodbav:"MessageContent"`
	NodeID         string    `dynamodbav:"NodeID,omitempty"`
	TTL            int64     `dynamodbav:"TTL,omitempty"`
}
