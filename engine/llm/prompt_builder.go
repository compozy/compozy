package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/compozy/compozy/engine/agent"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	orchestrator "github.com/compozy/compozy/engine/llm/orchestrator"
	"github.com/compozy/compozy/engine/llm/orchestrator/prompts"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	defaultTemplateName      = "action_prompt_default.tmpl"
	jsonFallbackTemplateName = "action_prompt_json_fallback.tmpl"
	jsonNativeTemplateName   = "action_prompt_json_native.tmpl"
)

// promptBuilder renders user prompts using reusable templates.
type promptBuilder struct {
	templates *template.Template
}

// templateStaticData captures invariant values for a rendered prompt.
type templateStaticData struct {
	ActionPrompt string
	HasTools     bool
	SchemaJSON   string
}

// templatePayload merges static and dynamic data prior to rendering.
type templatePayload struct {
	templateStaticData
	Examples        []orchestrator.PromptExample
	FailureGuidance []string
}

// promptTemplateState re-renders prompts when dynamic context changes.
type promptTemplateState struct {
	builder *promptBuilder
	variant string
	static  templateStaticData
}

// NewPromptBuilder creates a prompt builder backed by embedded templates.
func NewPromptBuilder() orchestrator.PromptBuilder {
	tmpl := template.Must(template.
		New("orchestrator_prompts").
		Funcs(template.FuncMap{}).
		ParseFS(prompts.TemplateFS, "templates/*.tmpl"))
	return &promptBuilder{templates: tmpl}
}

//nolint:gocritic // PromptBuildInput passed by value to keep builder free of shared mutable state.
func (b *promptBuilder) Build(
	ctx context.Context,
	input orchestrator.PromptBuildInput,
) (orchestrator.PromptBuildResult, error) {
	if input.Action == nil {
		return orchestrator.PromptBuildResult{}, fmt.Errorf("action config is nil")
	}

	static := templateStaticData{
		ActionPrompt: input.Action.Prompt,
		HasTools:     len(input.Tools) > 0,
	}

	variant := defaultTemplateName
	format := llmadapter.DefaultOutputFormat()

	if schema := input.Action.OutputSchema; schema != nil {
		if len(input.Tools) == 0 && shouldUseNativeStructured(input.ProviderCaps, input.Action) {
			variant = jsonNativeTemplateName
			format = llmadapter.NewJSONSchemaOutputFormat(outputSchemaName(input.Action), schema, true)
		} else {
			variant = jsonFallbackTemplateName
			static.SchemaJSON = marshalSchema(ctx, schema)
		}
	}

	rendered, normalized, err := b.render(variant, static, input.Dynamic)
	if err != nil {
		return orchestrator.PromptBuildResult{}, err
	}

	state := &promptTemplateState{
		builder: b,
		variant: variant,
		static:  static,
	}

	return orchestrator.PromptBuildResult{
		Prompt:   rendered,
		Format:   format,
		Template: state,
		Context:  normalized,
	}, nil
}

func (b *promptBuilder) render(
	variant string,
	static templateStaticData,
	dynamic orchestrator.PromptDynamicContext,
) (string, orchestrator.PromptDynamicContext, error) {
	if b.templates == nil {
		return "", orchestrator.PromptDynamicContext{}, fmt.Errorf("prompt templates not initialized")
	}
	tpl := b.templates.Lookup(variant)
	if tpl == nil {
		return "", orchestrator.PromptDynamicContext{}, fmt.Errorf("prompt template %s not found", variant)
	}
	payload := templatePayload{
		templateStaticData: static,
		Examples:           sanitizeExamples(dynamic.Examples),
		FailureGuidance:    sanitizeGuidance(dynamic.FailureGuidance),
	}
	normalized := orchestrator.PromptDynamicContext{
		Examples:        payload.Examples,
		FailureGuidance: payload.FailureGuidance,
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, payload); err != nil {
		return "", orchestrator.PromptDynamicContext{}, err
	}
	return strings.TrimSpace(buf.String()), normalized, nil
}

func (s *promptTemplateState) Render(
	_ context.Context,
	dynamic orchestrator.PromptDynamicContext,
) (string, error) {
	if s == nil || s.builder == nil {
		return "", fmt.Errorf("prompt template state is not initialized")
	}
	rendered, _, err := s.builder.render(s.variant, s.static, dynamic)
	return rendered, err
}

func shouldUseNativeStructured(
	caps llmadapter.ProviderCapabilities,
	action *agent.ActionConfig,
) bool {
	if !caps.StructuredOutput {
		return false
	}
	if action == nil {
		return false
	}
	return action.OutputSchema != nil || action.ShouldUseJSONOutput()
}

func outputSchemaName(action *agent.ActionConfig) string {
	if action == nil || strings.TrimSpace(action.ID) == "" {
		return "action_output"
	}
	return action.ID
}

func marshalSchema(ctx context.Context, schema any) string {
	if schema == nil {
		return ""
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		logger.FromContext(ctx).Error("Failed to marshal schema for structured output", "error", err)
		return "{}"
	}
	return string(encoded)
}

func sanitizeExamples(examples []orchestrator.PromptExample) []orchestrator.PromptExample {
	if len(examples) == 0 {
		return nil
	}
	result := make([]orchestrator.PromptExample, 0, len(examples))
	for _, example := range examples {
		summary := strings.TrimSpace(example.Summary)
		content := strings.TrimSpace(example.Content)
		if summary == "" && content == "" {
			continue
		}
		result = append(result, orchestrator.PromptExample{
			Summary: summary,
			Content: content,
		})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func sanitizeGuidance(guidance []string) []string {
	if len(guidance) == 0 {
		return nil
	}
	result := make([]string, 0, len(guidance))
	for _, item := range guidance {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
