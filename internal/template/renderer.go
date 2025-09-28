package template

import (
	"fmt"
	"log/slog"
	"strings"
)

type Renderer interface {
	Render(template string, data map[string]string) string
}

type renderer struct {
	logger *slog.Logger
}

func NewRenderer(logger *slog.Logger) Renderer {
	return &renderer{logger: logger.With("component", "renderer")}
}

func (r *renderer) Render(template string, data map[string]string) string {
	result := template

	for key, value := range data {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}

	r.logger.Debug("Rendered template",
		"data_keys", getKeys(data),
		"result", result,
	)

	return strings.TrimSpace(result)
}

func getKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	return keys
}
