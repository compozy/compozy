package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
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

type staticClient struct {
	resp *llmadapter.LLMResponse
	err  error
}

func (s *staticClient) GenerateContent(context.Context, *llmadapter.LLMRequest) (*llmadapter.LLMResponse, error) {
	return s.resp, s.err
}

func (s *staticClient) Close() error { return nil }

type capturingMetrics struct {
	outcomes         []string
	errorTypes       []string
	promptTokens     int
	completionTokens int
	costUSD          float64
}

func (c *capturingMetrics) RecordRequest(
	_ context.Context,
	_ enginecore.ProviderName,
	_ string,
	_ time.Duration,
	outcome string,
) {
	c.outcomes = append(c.outcomes, outcome)
}

func (c *capturingMetrics) RecordTokens(
	_ context.Context,
	_ enginecore.ProviderName,
	_ string,
	tokenType string,
	tokens int,
) {
	switch tokenType {
	case tokenTypePrompt:
		c.promptTokens += tokens
	case tokenTypeCompletion:
		c.completionTokens += tokens
	}
}

func (c *capturingMetrics) RecordCost(_ context.Context, _ enginecore.ProviderName, _ string, cost float64) {
	c.costUSD += cost
}

func (c *capturingMetrics) RecordError(_ context.Context, _ enginecore.ProviderName, _ string, errorType string) {
	c.errorTypes = append(c.errorTypes, errorType)
}

func (c *capturingMetrics) RecordRateLimitDelay(context.Context, enginecore.ProviderName, time.Duration) {
}

func TestLLMInvoker_Invoke(t *testing.T) {
	inv := NewLLMInvoker(&settings{
		retryAttempts:    3,
		retryBackoffBase: 1,
		retryBackoffMax:  1000000,
		providerMetrics:  providermetrics.Nop(),
	})
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

	inv := NewLLMInvoker(&settings{
		retryAttempts:    2,
		retryBackoffBase: time.Millisecond,
		retryBackoffMax:  time.Second,
		providerMetrics:  providermetrics.Nop(),
	})
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

func TestLLMInvoker_RecordsMetrics(t *testing.T) {
	metrics := &capturingMetrics{}
	inv := NewLLMInvoker(&settings{
		retryAttempts:    1,
		retryBackoffBase: time.Millisecond,
		retryBackoffMax:  time.Millisecond,
		providerMetrics:  metrics,
	})
	req := &llmadapter.LLMRequest{}
	response := &llmadapter.LLMResponse{
		Content: "ok",
		Usage: &llmadapter.Usage{
			PromptTokens:     2000,
			CompletionTokens: 1000,
		},
	}
	request := Request{
		Agent: &agent.Config{
			ID:    "agent-1",
			Model: agent.Model{Config: enginecore.ProviderConfig{Provider: enginecore.ProviderOpenAI, Model: "gpt-4o"}},
		},
		Action: &agent.ActionConfig{ID: "action", Prompt: "prompt"},
	}
	client := &staticClient{resp: response}
	resp, err := inv.Invoke(context.Background(), client, req, request)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Contains(t, metrics.outcomes, providerOutcomeSuccess)
	require.Equal(t, 2000, metrics.promptTokens)
	require.Equal(t, 1000, metrics.completionTokens)
	require.Greater(t, metrics.costUSD, 0.0)
}

func TestLLMInvoker_RecordsErrorMetrics(t *testing.T) {
	metrics := &capturingMetrics{}
	inv := NewLLMInvoker(&settings{
		retryAttempts:    1,
		retryBackoffBase: time.Millisecond,
		retryBackoffMax:  time.Millisecond,
		providerMetrics:  metrics,
	})
	client := &staticClient{
		err: llmadapter.NewErrorWithCode(llmadapter.ErrCodeBadRequest, "bad", string(enginecore.ProviderOpenAI), nil),
	}
	request := Request{
		Agent: &agent.Config{
			ID:    "agent-1",
			Model: agent.Model{Config: enginecore.ProviderConfig{Provider: enginecore.ProviderOpenAI, Model: "gpt-4o"}},
		},
		Action: &agent.ActionConfig{ID: "action", Prompt: "prompt"},
	}
	_, err := inv.Invoke(context.Background(), client, &llmadapter.LLMRequest{}, request)
	require.Error(t, err)
	require.Contains(t, metrics.outcomes, providerOutcomeError)
	require.Contains(t, metrics.errorTypes, errorTypeInvalid)
}
