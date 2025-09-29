package llm

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/schema"
	"github.com/stretchr/testify/assert"
)

func TestPromptBuilder_ShouldUseStructuredOutput(t *testing.T) {
	builder := NewPromptBuilder()

	t.Run("Should enable structured output for OpenAI when schema defined", func(t *testing.T) {
		action := &agent.ActionConfig{OutputSchema: &schema.Schema{"type": "object"}}
		result := builder.ShouldUseStructuredOutput("openai", action, nil)
		assert.True(t, result)
	})

	t.Run("Should disable structured output for Groq due to unsupported JSON schema", func(t *testing.T) {
		action := &agent.ActionConfig{OutputSchema: &schema.Schema{"type": "object"}}
		result := builder.ShouldUseStructuredOutput("groq", action, nil)
		assert.False(t, result)
	})
}
