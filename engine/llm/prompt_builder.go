package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
)

// PromptBuilder handles prompt construction and enhancement
type PromptBuilder interface {
	// Build constructs the main prompt from action config
	Build(ctx context.Context, action *agent.ActionConfig) (string, error)
	// EnhanceForStructuredOutput enhances prompt for structured JSON output
	EnhanceForStructuredOutput(ctx context.Context, prompt string, schema *schema.Schema, hasTools bool) string
	// ShouldUseStructuredOutput determines if structured output should be used
	ShouldUseStructuredOutput(provider string, action *agent.ActionConfig, tools []tool.Config) bool
}

// Implementation of Builder
type promptBuilder struct{}

// NewPromptBuilder creates a new prompt builder
func NewPromptBuilder() PromptBuilder {
	return &promptBuilder{}
}

// Build constructs the main prompt from action config
func (b *promptBuilder) Build(_ context.Context, action *agent.ActionConfig) (string, error) {
	if action == nil {
		return "", fmt.Errorf("action config is nil")
	}

	// For now, return the prompt directly
	// Future: add template processing, variable substitution, etc.
	return action.Prompt, nil
}

// EnhanceForStructuredOutput enhances prompt for structured JSON output
func (b *promptBuilder) EnhanceForStructuredOutput(
	ctx context.Context,
	prompt string,
	schema *schema.Schema,
	hasTools bool,
) string {
	log := logger.FromContext(ctx)
	if schema != nil {
		// Enhanced prompt with schema
		schemaJSON, err := json.Marshal(schema)
		if err != nil {
			// This is a developer error (bad schema), so we fallback to original prompt
			log.Error("Failed to marshal schema for structured output", "error", err)
			return prompt
		}
		return fmt.Sprintf(`%s

IMPORTANT: You MUST respond with a valid JSON object only that conforms to the `+
			`following schema:

%s

Do not include any text before or after the JSON object. `+
			`The response must be parseable JSON.`, prompt, string(schemaJSON))
	}

	if hasTools {
		// Enhanced prompt for tools without specific schema
		return fmt.Sprintf(`%s

IMPORTANT: You MUST respond in valid JSON format only. `+
			`Just use a tool call if needed, or return a JSON object with your response.`, prompt)
	}

	// No enhancement needed
	return prompt
}

// ShouldUseStructuredOutput determines if structured output should be used
func (b *promptBuilder) ShouldUseStructuredOutput(
	provider string,
	action *agent.ActionConfig,
	tools []tool.Config,
) bool {
	// Only certain providers support structured output
	if !b.supportsStructuredOutput(provider) {
		return false
	}

	// Check if action has JSON mode or JSON output schema
	if action != nil && (action.JSONMode || action.OutputSchema != nil) {
		return true
	}

	// Check if any tool has output schema
	for i := range tools {
		if tools[i].OutputSchema != nil {
			return true
		}
	}

	return false
}

// supportsStructuredOutput checks if the provider supports structured output
func (b *promptBuilder) supportsStructuredOutput(provider string) bool {
	switch provider {
	case "openai", "groq":
		return true
	default:
		return false
	}
}

// Future: Add more sophisticated prompt enhancement features
// - Template processing with variables
// - Context injection
// - Multi-step reasoning prompts
// - Chain-of-thought prompting
// - Custom prompt formatters
