package llm

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	orchestrator "github.com/compozy/compozy/engine/llm/orchestrator"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/require"
)

func TestPromptBuilder_NativeStructuredOutput(t *testing.T) {
	builder := NewPromptBuilder()
	ctx := t.Context()

	action := &agent.ActionConfig{
		ID:           "summarize",
		Prompt:       "Summarize the findings into JSON.",
		OutputSchema: &schema.Schema{"type": "object"},
	}

	result, err := builder.Build(ctx, orchestrator.PromptBuildInput{
		Action:       action,
		ProviderCaps: llmadapter.ProviderCapabilities{StructuredOutput: true},
	})
	require.NoError(t, err)
	require.True(t, result.Format.IsJSONSchema())
	require.Contains(t, result.Prompt, "You MUST respond with a valid JSON object")
}

func TestPromptBuilder_StructuredOutputFallback(t *testing.T) {
	builder := NewPromptBuilder()
	ctx := t.Context()

	action := &agent.ActionConfig{
		Prompt:       "Summarize the findings into JSON.",
		OutputSchema: &schema.Schema{"type": "object"},
	}

	result, err := builder.Build(ctx, orchestrator.PromptBuildInput{
		Action: action,
	})
	require.NoError(t, err)
	require.False(t, result.Format.IsJSONSchema())
	require.Contains(t, result.Prompt, "MUST respond with a valid JSON object only")
	require.Contains(t, result.Prompt, `"type":"object"`)
}

func TestPromptBuilder_ToolGuidance(t *testing.T) {
	builder := NewPromptBuilder()
	ctx := t.Context()

	action := &agent.ActionConfig{
		Prompt: "Respond to the user.",
	}

	result, err := builder.Build(ctx, orchestrator.PromptBuildInput{
		Action: action,
		Tools:  []tool.Config{{ID: "cp__call_agent"}},
	})
	require.NoError(t, err)
	require.Contains(t, result.Prompt, "Use the tool call format only when invoking a tool")
	require.Contains(t, result.Prompt, "Provide the final response as plain text")
}

func TestPromptBuilder_DynamicFailureGuidance(t *testing.T) {
	builder := NewPromptBuilder()
	ctx := t.Context()

	action := &agent.ActionConfig{
		Prompt: "Provide an answer.",
	}

	result, err := builder.Build(ctx, orchestrator.PromptBuildInput{
		Action: action,
	})
	require.NoError(t, err)

	rendered, err := result.Template.Render(ctx, orchestrator.PromptDynamicContext{
		FailureGuidance: []string{"Observation: tool cp__call_agent failed."},
	})
	require.NoError(t, err)
	require.Contains(t, rendered, "Observation: tool cp__call_agent failed.")
}
