package llm

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/schema"
	"github.com/stretchr/testify/require"
)

func TestLLMOrchestratorParseContent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	orchestrator := &llmOrchestrator{}
	outputSchema := schema.Schema{
		"type": "object",
		"properties": map[string]any{
			"weather":     map[string]any{"type": "string"},
			"temperature": map[string]any{"type": "number"},
		},
		"required": []any{"weather", "temperature"},
	}
	action := &agent.ActionConfig{ID: "structured", OutputSchema: &outputSchema}
	t.Run("ShouldReturnOutputWhenJSONMatchesSchema", func(t *testing.T) {
		content := `{"weather":"sunny","temperature":23}`
		result, err := orchestrator.parseContent(ctx, content, action)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "sunny", (*result)["weather"])
	})
	t.Run("ShouldFailWhenJSONMissingRequiredFields", func(t *testing.T) {
		content := `{"weather":"sunny"}`
		result, err := orchestrator.parseContent(ctx, content, action)
		require.Nil(t, result)
		require.Error(t, err)
		var coreErr *core.Error
		require.ErrorAs(t, err, &coreErr)
		require.Equal(t, ErrCodeInputValidation, coreErr.Code)
	})
}

func TestHandleNoToolCallsStructuredOutputRetry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	outputSchema := schema.Schema{
		"type": "object",
		"properties": map[string]any{
			"answer": map[string]any{"type": "string"},
		},
		"required": []any{"answer"},
	}
	action := &agent.ActionConfig{ID: "retry-action", OutputSchema: &outputSchema}
	request := Request{Agent: &agent.Config{ID: "agent"}, Action: action}
	orchestrator := &llmOrchestrator{config: OrchestratorConfig{StructuredOutputRetryAttempts: 2}}
	counters := map[string]int{}
	llmReq := &llmadapter.LLMRequest{}
	response := &llmadapter.LLMResponse{Content: `{"unexpected":"value"}`}
	structuredBudget := orchestrator.effectiveStructuredOutputRetries()
	require.Equal(t, 2, structuredBudget)
	output, cont, err := orchestrator.handleNoToolCalls(ctx, response, request, llmReq, counters, 8, nil)
	require.Nil(t, output)
	require.True(t, cont)
	require.NoError(t, err)
	require.Len(t, llmReq.Messages, 2)
	require.Equal(t, 1, counters["output_validator"])
	output, cont, err = orchestrator.handleNoToolCalls(ctx, response, request, llmReq, counters, 8, nil)
	require.Nil(t, output)
	require.False(t, cont)
	require.Error(t, err)
	require.ErrorContains(t, err, "output_validator")
	require.ErrorContains(t, err, "budget exceeded")
}
