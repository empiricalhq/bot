// Package whatsbot carries the conversation flow this binary was built with.
package whatsbot

import _ "embed"

// DefaultFlow is the repository's conversation.json as it was at build time.
//
//go:embed conversation.json
var DefaultFlow []byte
