package orchestrator

import (
	"context"
	"strings"
	"testing"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/require"
)

func TestOnEnterUpdateBudgets_InjectsCompletionHintForAgentCalls(t *testing.T) {
	t.Run("ShouldInjectCompletionHintAfterSuccessfulAgentCall", func(t *testing.T) {
		ctx := context.Background()
		settings := &settings{
			enableAgentCallCompletionHints: true,
			maxSequentialToolErrors:        5,
			maxConsecutiveSuccesses:        3,
		}
		toolExec := &recordingToolExecutor{}
		loop := newConversationLoop(settings, toolExec, noopResponseHandler{}, &recordingInvoker{}, nil)
		state := newLoopState(settings, nil, nil)
		loopCtx := &LoopContext{
			LLMRequest: &llmadapter.LLMRequest{Messages: []llmadapter.Message{}},
			ToolResults: []llmadapter.ToolResult{
				{
					Name:    "cp__call_agent",
					Content: `{"agent_id":"researcher","response":{"answer":"test"}}`,
				},
			},
			State: state,
		}
		result := loop.OnEnterUpdateBudgets(ctx, loopCtx)
		require.Equal(t, EventBudgetOK, result.Event)
		require.NoError(t, result.Err)
		require.Len(t, loopCtx.LLMRequest.Messages, 2)
		require.Equal(t, roleTool, loopCtx.LLMRequest.Messages[0].Role)
		require.Equal(t, "user", loopCtx.LLMRequest.Messages[1].Role)
		require.Contains(t, loopCtx.LLMRequest.Messages[1].Content, "researcher")
		require.Contains(t, loopCtx.LLMRequest.Messages[1].Content, "successfully completed")
		require.Contains(t, loopCtx.LLMRequest.Messages[1].Content, "final answer")
	})
	t.Run("ShouldNotInjectHintWhenFeatureDisabled", func(t *testing.T) {
		ctx := context.Background()
		settings := &settings{
			enableAgentCallCompletionHints: false,
			maxSequentialToolErrors:        5,
			maxConsecutiveSuccesses:        3,
		}
		toolExec := &recordingToolExecutor{}
		loop := newConversationLoop(settings, toolExec, noopResponseHandler{}, &recordingInvoker{}, nil)
		state := newLoopState(settings, nil, nil)
		loopCtx := &LoopContext{
			LLMRequest: &llmadapter.LLMRequest{Messages: []llmadapter.Message{}},
			ToolResults: []llmadapter.ToolResult{
				{
					Name:    "cp__call_agent",
					Content: `{"agent_id":"researcher","response":{"answer":"test"}}`,
				},
			},
			State: state,
		}
		result := loop.OnEnterUpdateBudgets(ctx, loopCtx)
		require.Equal(t, EventBudgetOK, result.Event)
		require.NoError(t, result.Err)
		require.Len(t, loopCtx.LLMRequest.Messages, 1)
		require.Equal(t, roleTool, loopCtx.LLMRequest.Messages[0].Role)
	})
	t.Run("ShouldNotInjectHintForNonAgentTools", func(t *testing.T) {
		ctx := context.Background()
		settings := &settings{
			enableAgentCallCompletionHints: true,
			maxSequentialToolErrors:        5,
			maxConsecutiveSuccesses:        3,
		}
		toolExec := &recordingToolExecutor{}
		loop := newConversationLoop(settings, toolExec, noopResponseHandler{}, &recordingInvoker{}, nil)
		state := newLoopState(settings, nil, nil)
		loopCtx := &LoopContext{
			LLMRequest: &llmadapter.LLMRequest{Messages: []llmadapter.Message{}},
			ToolResults: []llmadapter.ToolResult{
				{
					Name:    "cp__read_file",
					Content: `{"content":"test"}`,
				},
			},
			State: state,
		}
		result := loop.OnEnterUpdateBudgets(ctx, loopCtx)
		require.Equal(t, EventBudgetOK, result.Event)
		require.NoError(t, result.Err)
		require.Len(t, loopCtx.LLMRequest.Messages, 1)
		require.Equal(t, roleTool, loopCtx.LLMRequest.Messages[0].Role)
	})
	t.Run("ShouldNotInjectHintForFailedAgentCall", func(t *testing.T) {
		ctx := context.Background()
		settings := &settings{
			enableAgentCallCompletionHints: true,
			maxSequentialToolErrors:        5,
			maxConsecutiveSuccesses:        3,
		}
		toolExec := &recordingToolExecutor{}
		loop := newConversationLoop(settings, toolExec, noopResponseHandler{}, &recordingInvoker{}, nil)
		state := newLoopState(settings, nil, nil)
		loopCtx := &LoopContext{
			LLMRequest: &llmadapter.LLMRequest{Messages: []llmadapter.Message{}},
			ToolResults: []llmadapter.ToolResult{
				{
					Name:    "cp__call_agent",
					Content: `{"error":"agent not found"}`,
				},
			},
			State: state,
		}
		result := loop.OnEnterUpdateBudgets(ctx, loopCtx)
		require.Equal(t, EventBudgetOK, result.Event)
		hasCompletionHint := false
		for _, msg := range loopCtx.LLMRequest.Messages {
			if msg.Role == "user" && msg.Content != "" &&
				(strings.Contains(msg.Content, "successfully completed") ||
					strings.Contains(msg.Content, "final answer")) {
				hasCompletionHint = true
				break
			}
		}
		require.False(t, hasCompletionHint, "should not inject completion hint for failed agent call")
	})
}

type recordingToolExecutor struct{}

func (r *recordingToolExecutor) Execute(
	_ context.Context,
	_ []llmadapter.ToolCall,
) ([]llmadapter.ToolResult, error) {
	return nil, nil
}

func (r *recordingToolExecutor) UpdateBudgets(
	_ context.Context,
	_ []llmadapter.ToolResult,
	_ *loopState,
) error {
	return nil
}
