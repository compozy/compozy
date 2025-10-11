package orchestrator

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/telemetry"
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

const defaultRetryJitter = 50 * time.Millisecond

func NewLLMInvoker(cfg *settings) LLMInvoker {
	if cfg == nil {
		cfg = &settings{}
	}
	return &llmInvoker{cfg: *cfg}
}

//nolint:gocritic // Request is copied intentionally to snapshot invocation metadata for retries.
func (i *llmInvoker) Invoke(
	ctx context.Context,
	client llmadapter.LLMClient,
	req *llmadapter.LLMRequest,
	request Request,
) (*llmadapter.LLMResponse, error) {
	callStarted := time.Now()
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

	if attempts <= 0 || attempts > 100 {
		attempts = defaultRetryAttempts
	}

	maxRetries := uint64(attempts) // #nosec G115 -- attempts sanitized above
	var backoff retry.Backoff
	if i.cfg.retryJitter {
		backoff = retry.WithMaxRetries(maxRetries, retry.WithJitter(defaultRetryJitter, exponential))
	} else {
		backoff = retry.WithMaxRetries(maxRetries, exponential)
	}

	callAttempts := 0
	var lastErr error
	var response *llmadapter.LLMResponse
	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		callAttempts++
		var callErr error
		response, callErr = client.GenerateContent(ctx, req)
		if callErr != nil {
			lastErr = callErr
			if isRetryableErrorWithContext(ctx, callErr) {
				return retry.RetryableError(callErr)
			}
			return callErr
		}
		lastErr = nil
		return nil
	})
	callDuration := time.Since(callStarted)
	i.recordProviderCall(ctx, &request, callAttempts, callDuration, lastErr)
	if err != nil {
		return nil, NewLLMError(err, ErrCodeLLMGeneration, map[string]any{
			"agent":  request.Agent.ID,
			"action": request.Action.ID,
		})
	}
	return response, nil
}

func (i *llmInvoker) recordProviderCall(
	ctx context.Context,
	request *Request,
	attempts int,
	duration time.Duration,
	err error,
) {
	metadata := map[string]any{
		"latency_ms": float64(duration) / float64(time.Millisecond),
		"attempts":   attempts,
		"retry_cap":  i.cfg.retryAttempts,
	}
	if i.cfg.timeout > 0 {
		metadata["timeout_ms"] = float64(i.cfg.timeout) / float64(time.Millisecond)
	}
	if request != nil && request.Agent != nil {
		metadata["agent_id"] = request.Agent.ID
		if provider := request.Agent.Model.Config.Provider; provider != "" {
			metadata["provider"] = provider
		}
		if model := request.Agent.Model.Config.Model; model != "" {
			metadata["model"] = model
		}
	}
	if request != nil && request.Action != nil {
		metadata["action_id"] = request.Action.ID
	}
	event := telemetry.Event{
		Stage:    "provider_call",
		Severity: telemetry.SeverityInfo,
		Metadata: metadata,
	}
	if err != nil {
		event.Severity = telemetry.SeverityError
		event.Metadata["error"] = core.RedactError(err)
	}
	telemetry.RecordEvent(ctx, &event)
}
