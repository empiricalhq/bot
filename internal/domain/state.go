package domain

import "time"

type UserState struct {
	UserID             string    `db:"user_id"`
	CurrentNode        string    `db:"current_node"`
	UserName           string    `db:"user_name"`
	CourseInterest     string    `db:"course_interest"`
	SelectedCourseID   string    `db:"selected_course_id"`
	ConsultedPrice     bool      `db:"consulted_price"`
	VoucherPath        string    `db:"voucher_path"`
	RequiresHumanAgent bool      `db:"requires_human_agent"`
	RepromptCount      int       `db:"reprompt_count"`
	LastUpdated        time.Time `db:"last_updated"`
}

type ConversationMessage struct {
	UserID         string    `db:"user_id"`
	Timestamp      time.Time `db:"timestamp"`
	Direction      string    `db:"direction"`
	MessageContent string    `db:"message_content"`
	NodeID         string    `db:"node_id"`
}
