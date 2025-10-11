package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type flappyClient struct {
	calls    int
	finalErr error
}

func (f *flappyClient) GenerateContent(context.Context, *llmadapter.LLMRequest) (*llmadapter.LLMResponse, error) {
	f.calls++
	if f.calls < 2 {
		return nil, llmadapter.NewErrorWithCode(llmadapter.ErrCodeTimeout, "t", "p", nil)
	}
	if f.finalErr != nil {
		return nil, f.finalErr
	}
	return &llmadapter.LLMResponse{Content: "ok"}, nil
}
func (f *flappyClient) Close() error { return nil }

func TestLLMInvoker_Invoke(t *testing.T) {
	inv := NewLLMInvoker(&settings{retryAttempts: 3, retryBackoffBase: 1, retryBackoffMax: 1000000})
	req := &llmadapter.LLMRequest{}
	t.Run("Should retry on retryable errors", func(t *testing.T) {
		c := &flappyClient{}
		resp, err := inv.Invoke(
			context.Background(),
			c,
			req,
			Request{
				Agent: &agent.Config{
					ID:    "ag",
					Model: agent.Model{Config: enginecore.ProviderConfig{Provider: "openai", Model: "m"}},
				},
				Action: &agent.ActionConfig{ID: "act", Prompt: "p"},
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "ok", resp.Content)
		assert.GreaterOrEqual(t, c.calls, 2)
	})
	t.Run("Should not retry on non-retryable", func(t *testing.T) {
		c := &flappyClient{finalErr: llmadapter.NewErrorWithCode(llmadapter.ErrCodeBadRequest, "b", "p", nil)}
		_, err := inv.Invoke(
			context.Background(),
			c,
			req,
			Request{
				Agent: &agent.Config{
					ID:    "ag",
					Model: agent.Model{Config: enginecore.ProviderConfig{Provider: "openai", Model: "m"}},
				},
				Action: &agent.ActionConfig{ID: "act", Prompt: "p"},
			},
		)
		require.Error(t, err)
	})
}

func TestLLMInvoker_RecordsProviderTelemetry(t *testing.T) {
	tempDir := t.TempDir()
	recorder, err := telemetry.NewRecorder(&telemetry.Options{ProjectRoot: tempDir})
	require.NoError(t, err)
	ctx, run, err := recorder.StartRun(context.Background(), telemetry.RunMetadata{})
	require.NoError(t, err)

	inv := NewLLMInvoker(&settings{retryAttempts: 2, retryBackoffBase: time.Millisecond, retryBackoffMax: time.Second})
	client := &flappyClient{}
	request := Request{
		Agent: &agent.Config{
			ID:    "agent-1",
			Model: agent.Model{Config: enginecore.ProviderConfig{Provider: "openai", Model: "gpt"}},
		},
		Action: &agent.ActionConfig{ID: "action-1", Prompt: "do it"},
	}
	resp, callErr := inv.Invoke(ctx, client, &llmadapter.LLMRequest{}, request)
	require.NoError(t, callErr)
	require.NotNil(t, resp)

	closeErr := recorder.CloseRun(ctx, run, telemetry.RunResult{Success: true})
	require.NoError(t, closeErr)

	events := readRunEvents(t, tempDir)
	evt, ok := findEventByStage(events, "provider_call")
	require.True(t, ok, "provider_call telemetry event missing")

	metadata, ok := evt["metadata"].(map[string]any)
	require.True(t, ok)
	latency, ok := metadata["latency_ms"].(float64)
	require.True(t, ok)
	require.Greater(t, latency, 0.0)
	attempts, ok := metadata["attempts"].(float64)
	require.True(t, ok)
	require.GreaterOrEqual(t, int(attempts), 1)
	provider, ok := metadata["provider"].(string)
	require.True(t, ok)
	require.Equal(t, "openai", provider)
}
