package orchestrator

import (
	"context"
	"testing"
	"time"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/require"
)

type retryAfterClient struct {
	delay     time.Duration
	attempts  int
	succeeded bool
}

func (c *retryAfterClient) GenerateContent(
	_ context.Context,
	_ *llmadapter.LLMRequest,
) (*llmadapter.LLMResponse, error) {
	c.attempts++
	if c.attempts == 1 {
		return nil, llmadapter.
			NewErrorWithCode(llmadapter.ErrCodeRateLimit, "rate limited", "test", nil).
			WithRetryAfter(c.delay)
	}
	c.succeeded = true
	return &llmadapter.LLMResponse{Content: "ok"}, nil
}

func (c *retryAfterClient) Close() error { return nil }

func TestLLMInvoker(t *testing.T) {
	t.Parallel()

	t.Run("Should respect retry-after delay", func(t *testing.T) {
		t.Parallel()

		delay := 35 * time.Millisecond
		client := &retryAfterClient{delay: delay}
		invoker := NewLLMInvoker(&settings{
			retryAttempts:      2,
			retryBackoffBase:   time.Millisecond,
			retryBackoffMax:    100 * time.Millisecond,
			retryJitter:        false,
			maxConcurrentTools: 0,
		})

		ctx := t.Context()
		start := time.Now()
		resp, err := invoker.Invoke(ctx, client, &llmadapter.LLMRequest{}, Request{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, client.succeeded)
		require.GreaterOrEqual(t, client.attempts, 2)

		elapsed := time.Since(start)
		require.GreaterOrEqual(t, elapsed, delay)
		require.Less(t, elapsed, delay+250*time.Millisecond)
	})
}
