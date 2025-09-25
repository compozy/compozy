package orchestrator

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
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
