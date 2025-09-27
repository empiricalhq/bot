package template

import (
	"log/slog"
	"strings"

	"whatsbot/internal/domain"
	"whatsbot/internal/nameparser"
)

type Renderer interface {
	Render(template string, state *domain.UserState) string
}

type renderer struct {
	logger *slog.Logger
}

func NewRenderer(logger *slog.Logger) Renderer {
	return &renderer{logger: logger.With("component", "renderer")}
}

func (r *renderer) Render(template string, state *domain.UserState) string {
	name := nameparser.Parse(state.UserName)
	if name == "" {
		name = "amigx"
	}

	result := strings.ReplaceAll(template, "{{name}}", name)

	r.logger.Debug("Rendered template",
		"user", state.UserID,
		"user_name_input", state.UserName,
		"parsed_name", name,
		"result", result,
	)

	return strings.TrimSpace(result)
}
