package llm

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	orchestrator "github.com/compozy/compozy/engine/llm/orchestrator"
	"github.com/compozy/compozy/engine/llm/orchestrator/prompts"
)

type systemPromptRenderer struct {
	template *template.Template
}

type systemPromptData struct {
	HasInstructions bool
	Instructions    string
}

// NewSystemPromptRenderer constructs a renderer for system prompts that include built-in tool guidance.
func NewSystemPromptRenderer() orchestrator.SystemPromptRenderer {
	tpl := template.Must(
		template.New("system_prompt_renderer").
			ParseFS(prompts.TemplateFS, "templates/system_prompt_with_builtins.tmpl"),
	)
	return &systemPromptRenderer{
		template: tpl.Lookup("system_prompt_with_builtins.tmpl"),
	}
}

func (r *systemPromptRenderer) Render(_ context.Context, instructions string) (string, error) {
	if r == nil || r.template == nil {
		return "", fmt.Errorf("system prompt template not initialized")
	}
	data := systemPromptData{
		HasInstructions: strings.TrimSpace(instructions) != "",
		Instructions:    instructions,
	}
	var buf bytes.Buffer
	if err := r.template.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render system prompt: %w", err)
	}
	if _, err := buf.WriteString("\n"); err != nil {
		return "", fmt.Errorf("append system prompt newline: %w", err)
	}
	return buf.String(), nil
}
