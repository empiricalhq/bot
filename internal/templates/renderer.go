package templates

import "strings"

type Renderer interface {
	RenderText(template string, userName string) string
}

type TextRenderer struct{}

func NewTextRenderer() *TextRenderer {
	return &TextRenderer{}
}

func (r *TextRenderer) RenderText(template, userName string) string {
	if userName == "" {
		userName = "Amig@"
	}

	return strings.ReplaceAll(template, "{{name}}", userName)
}
