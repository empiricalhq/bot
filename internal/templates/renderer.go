package templates

import (
	"strings"

	"whatsbot/internal/domain"
)

// Renderer defines an interface for rendering response templates.
type Renderer interface {
	RenderText(template string, state *domain.UserState) string
}

// TextRenderer is a simple string-replacement renderer.
type TextRenderer struct{}

// NewTextRenderer creates a new text renderer.
func NewTextRenderer() *TextRenderer {
	return &TextRenderer{}
}

// RenderText replaces placeholders in a template with values from the user's state.
func (r *TextRenderer) RenderText(template string, state *domain.UserState) string {
	name := state.UserName
	if name == "" {
		name = "amigx" // A friendly default if name is not set.
	}

	result := strings.ReplaceAll(template, "{{name}}", name)

	return strings.TrimSpace(result)
}
