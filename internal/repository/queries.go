package repository

const (
	queryGetUserState = `
		SELECT current_node, user_name, course_interest, consulted_price, requires_human_agent, last_updated
		FROM user_state WHERE user_id = ?
	`

	querySaveUserState = `
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

	querySaveMessage = `
		INSERT INTO conversation_history (user_id, timestamp, direction, message_content, node_id)
		VALUES (?, ?, ?, ?, ?)
	`
)
