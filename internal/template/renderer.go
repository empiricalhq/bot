package template

import (
	"strings"

	"whatsbot/internal/domain"
	"whatsbot/internal/nameparser"
)

type Renderer interface {
	Render(template string, state *domain.UserState) string
}

type renderer struct{}

func NewRenderer() Renderer {
	return &renderer{}
}

func (r *renderer) Render(template string, state *domain.UserState) string {
	name := nameparser.Parse(state.UserName)
	if name == "" {
		name = "amigx"
	}

	result := strings.ReplaceAll(template, "{{name}}", name)

	return strings.TrimSpace(result)
}
