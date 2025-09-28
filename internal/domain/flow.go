package domain

type MessageContent struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type Condition struct {
	Type  string   `json:"type"`
	Value []string `json:"value,omitempty"`
	Regex string   `json:"regex,omitempty"`
}

type Transition struct {
	Condition Condition `json:"condition"`
	Target    string    `json:"target"`
	Action    string    `json:"action,omitempty"`
}

type Node struct {
	Title                   string         `json:"title,omitempty"`
	Message                 MessageContent `json:"message"`
	Transitions             []Transition   `json:"transitions,omitempty"`
	IncludeTransitions      string         `json:"include_transitions,omitempty"`
	Action                  string         `json:"action,omitempty"`
	IgnoreGlobalTransitions bool           `json:"ignore_global_transitions,omitempty"`
	FallbackMessage         string         `json:"fallback_message,omitempty"`
}

type Flow struct {
	StartNode         string                  `json:"start_node"`
	Nodes             map[string]Node         `json:"nodes"`
	GlobalTransitions []Transition            `json:"global_transitions,omitempty"`
	TransitionGroups  map[string][]Transition `json:"transition_groups,omitempty"`
}
