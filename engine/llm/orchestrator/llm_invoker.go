package orchestrator

import (
	"context"
	"time"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/sethvargo/go-retry"
)

type LLMInvoker interface {
	Invoke(
		ctx context.Context,
		client llmadapter.LLMClient,
		req *llmadapter.LLMRequest,
		request Request,
	) (*llmadapter.LLMResponse, error)
}

type llmInvoker struct {
	cfg settings
}

func NewLLMInvoker(cfg *settings) LLMInvoker {
	if cfg == nil {
		cfg = &settings{}
	}
	return &llmInvoker{cfg: *cfg}
}

func (i *llmInvoker) Invoke(
	ctx context.Context,
	client llmadapter.LLMClient,
	req *llmadapter.LLMRequest,
	request Request,
) (*llmadapter.LLMResponse, error) {
	if i.cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, i.cfg.timeout)
		defer cancel()
	}

	attempts := i.cfg.retryAttempts
	backoffBase := i.cfg.retryBackoffBase
	backoffMax := i.cfg.retryBackoffMax
	exponential := retry.NewExponential(backoffBase)
	exponential = retry.WithMaxDuration(backoffMax, exponential)

	if attempts <= 0 {
		attempts = defaultRetryAttempts
	}
	if attempts < 0 || attempts > 100 {
		attempts = defaultRetryAttempts
	}

	maxRetries := uint64(attempts) // #nosec G115 -- attempts sanitized above
	var backoff retry.Backoff
	if i.cfg.retryJitter {
		backoff = retry.WithMaxRetries(maxRetries, retry.WithJitter(50*time.Millisecond, exponential))
	} else {
		backoff = retry.WithMaxRetries(maxRetries, exponential)
	}

	var response *llmadapter.LLMResponse
	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		var callErr error
		response, callErr = client.GenerateContent(ctx, req)
		if callErr != nil {
			if isRetryableErrorWithContext(ctx, callErr) {
				return retry.RetryableError(callErr)
			}
			return callErr
		}
		return nil
	})
	if err != nil {
		return nil, NewLLMError(err, ErrCodeLLMGeneration, map[string]any{
			"agent":  request.Agent.ID,
			"action": request.Action.ID,
		})
	}
	return response, nil
}
