package orchestrator

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestComputeContextUsage_UsesProviderContextWindow(t *testing.T) {
	t.Run("Should use provider context window limit", func(t *testing.T) {
		t.Helper()
		loopCtx := &LoopContext{
			Request: Request{
				ProviderCaps: llmadapter.ProviderCapabilities{ContextWindowTokens: 1000},
			},
			LLMRequest: &llmadapter.LLMRequest{},
		}
		resp := &llmadapter.LLMResponse{
			Usage: &llmadapter.Usage{
				PromptTokens:     300,
				CompletionTokens: 100,
			},
		}
		usage := computeContextUsage(loopCtx, resp)
		require.Equal(t, 400, usage.TotalTokens)
		require.Equal(t, 1000, usage.ContextLimit)
		require.Equal(t, "provider", usage.LimitSource)
		require.InDelta(t, 0.4, usage.PercentOfLimit, 1e-9)
	})
}

func TestComputeContextUsage_FallsBackToAgentMaxTokens(t *testing.T) {
	t.Run("Should fall back to agent max tokens limit", func(t *testing.T) {
		t.Helper()
		loopCtx := &LoopContext{
			Request: Request{
				ProviderCaps: llmadapter.ProviderCapabilities{},
				Agent: &agent.Config{
					Model: agent.Model{
						Config: core.ProviderConfig{
							Params: core.PromptParams{MaxTokens: 256},
						},
					},
				},
			},
			LLMRequest: &llmadapter.LLMRequest{},
		}
		resp := &llmadapter.LLMResponse{
			Usage: &llmadapter.Usage{
				PromptTokens:     80,
				CompletionTokens: 64,
			},
		}
		usage := computeContextUsage(loopCtx, resp)
		require.Equal(t, 144, usage.TotalTokens)
		require.Equal(t, 256, usage.ContextLimit)
		require.Equal(t, "agent_max_tokens", usage.LimitSource)
		require.InDelta(t, 144.0/256.0, usage.PercentOfLimit, 1e-9)
	})
}

func TestComputeContextUsage_FallsBackToRequestOptions(t *testing.T) {
	t.Run("Should fall back to request options limit", func(t *testing.T) {
		t.Helper()
		loopCtx := &LoopContext{
			Request: Request{
				ProviderCaps: llmadapter.ProviderCapabilities{},
			},
			LLMRequest: &llmadapter.LLMRequest{
				Options: llmadapter.CallOptions{MaxTokens: 128},
			},
		}
		resp := &llmadapter.LLMResponse{
			Usage: &llmadapter.Usage{
				PromptTokens:     60,
				CompletionTokens: 20,
			},
		}
		usage := computeContextUsage(loopCtx, resp)
		require.Equal(t, 80, usage.TotalTokens)
		require.Equal(t, 128, usage.ContextLimit)
		require.Equal(t, "request_max_tokens", usage.LimitSource)
		require.InDelta(t, 80.0/128.0, usage.PercentOfLimit, 1e-9)
	})
}

func TestComputeContextUsage_WhenUnknownLimit(t *testing.T) {
	t.Run("Should handle unknown context limit", func(t *testing.T) {
		t.Helper()
		loopCtx := &LoopContext{
			Request:    Request{},
			LLMRequest: &llmadapter.LLMRequest{},
		}
		resp := &llmadapter.LLMResponse{
			Usage: &llmadapter.Usage{
				PromptTokens:     40,
				CompletionTokens: 20,
			},
		}
		usage := computeContextUsage(loopCtx, resp)
		require.Equal(t, 60, usage.TotalTokens)
		require.Equal(t, 0, usage.ContextLimit)
		require.Equal(t, "unknown", usage.LimitSource)
		require.Zero(t, usage.PercentOfLimit)
	})
}

func TestRecordLLMResponse_WarnsWhenContextLimitUnknown(t *testing.T) {
	t.Run("Should warn when context limit is unknown", func(t *testing.T) {
		t.Helper()
		cfg := settings{}
		loop := &conversationLoop{cfg: cfg}
		loopCtx := &LoopContext{
			Request: Request{
				Agent: &agent.Config{
					ID: "agent-id",
					Model: agent.Model{
						Config: core.ProviderConfig{
							Provider: core.ProviderMock,
						},
					},
				},
				Action: &agent.ActionConfig{ID: "action-id"},
			},
			LLMRequest: &llmadapter.LLMRequest{},
			State:      newLoopState(&cfg, nil, nil),
		}
		resp := &llmadapter.LLMResponse{
			Usage: &llmadapter.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
			},
		}
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		respCtx := telemetry.ContextWithRecorder(ctx, telemetry.NopRecorder())
		require.False(t, loopCtx.contextLimitWarned)
		loop.recordLLMResponse(respCtx, loopCtx, resp)
		require.True(t, loopCtx.contextLimitWarned)
	})
}
