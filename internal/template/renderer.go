package template

import (
	"strings"

	"whatsbot/internal/domain"
)

type Renderer interface {
	Render(template string, state *domain.UserState) string
}

type renderer struct{}

func NewRenderer() Renderer {
	return &renderer{}
}

func (r *renderer) Render(template string, state *domain.UserState) string {
	name := state.UserName
	if name == "" {
		name = "amigx"
	}

	result := strings.ReplaceAll(template, "{{name}}", name)

	return strings.TrimSpace(result)
}
