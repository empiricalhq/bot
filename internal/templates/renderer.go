package templates

import (
	"strings"

	"whatsbot/internal/domain"
)

type Renderer interface {
	RenderText(template string, state *domain.UserState) string
}

type TextRenderer struct{}

func NewTextRenderer() *TextRenderer {
	return &TextRenderer{}
}

// RenderText replaces placeholders in a template with values from the user's state.
func (r *TextRenderer) RenderText(template string, state *domain.UserState) string {
	name := state.UserName
	if name == "" {
		name = "amigx"
	}

	result := strings.ReplaceAll(template, "{{name}}", name)

	return strings.TrimSpace(result)
}
