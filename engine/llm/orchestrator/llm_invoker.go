package orchestrator

import (
	"context"
	"sync"
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

func (i *llmInvoker) applyTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if i.cfg.timeout <= 0 {
		return ctx, nil
	}
	ctx, cancel := context.WithTimeout(ctx, i.cfg.timeout)
	return ctx, cancel
}

func (i *llmInvoker) retryAttemptCount() uint64 {
	attempts := i.cfg.retryAttempts
	if attempts <= 0 || attempts > 100 {
		attempts = defaultRetryAttempts
	}
	return uint64(attempts) // #nosec G115 -- sanitized earlier
}

func (i *llmInvoker) buildBackoff(maxRetries uint64) retry.Backoff {
	exponential := retry.NewExponential(i.cfg.retryBackoffBase)
	exponential = retry.WithMaxDuration(i.cfg.retryBackoffMax, exponential)
	if i.cfg.retryJitter {
		return retry.WithMaxRetries(maxRetries, retry.WithJitter(defaultRetryJitter, exponential))
	}
	return retry.WithMaxRetries(maxRetries, exponential)
}

func (i *llmInvoker) invokeOnce(
	ctx context.Context,
	client llmadapter.LLMClient,
	req *llmadapter.LLMRequest,
	request *Request,
	backoff *adaptiveBackoff,
	response **llmadapter.LLMResponse,
	lastErr *error,
) error {
	releaseLimiter, acquireErr := i.acquireRateLimiter(ctx, request, backoff, lastErr)
	if acquireErr != nil {
		return acquireErr
	}
	tokensUsed := 0
	if releaseLimiter != nil {
		defer func() {
			releaseLimiter(tokensUsed)
		}()
	}
	resp, callErr := client.GenerateContent(ctx, req)
	if callErr != nil {
		if lastErr != nil {
			*lastErr = callErr
		}
		if isRetryableErrorWithContext(ctx, callErr) {
			applyRetryAfterHint(callErr, backoff)
			return retry.RetryableError(callErr)
		}
		return callErr
	}
	if resp != nil && resp.Usage != nil {
		tokensUsed = max(resp.Usage.TotalTokens, 0)
	}
	if lastErr != nil {
		*lastErr = nil
	}
	if response != nil {
		*response = resp
	}
	return nil
}

//nolint:gocritic // Request is copied intentionally to snapshot invocation metadata for retries.
func (i *llmInvoker) Invoke(
	ctx context.Context,
	client llmadapter.LLMClient,
	req *llmadapter.LLMRequest,
	request Request,
) (*llmadapter.LLMResponse, error) {
	callStarted := time.Now()
	ctx, cancel := i.applyTimeout(ctx)
	if cancel != nil {
		defer cancel()
	}

	maxRetries := i.retryAttemptCount()
	backoff := i.buildBackoff(maxRetries)
	rateLimitedBackoff := newAdaptiveBackoff(backoff)

	attempts := 0
	var lastErr error
	var response *llmadapter.LLMResponse
	err := retry.Do(ctx, rateLimitedBackoff, func(ctx context.Context) error {
		attempts++
		return i.invokeOnce(ctx, client, req, &request, rateLimitedBackoff, &response, &lastErr)
	})
	callDuration := time.Since(callStarted)
	i.recordProviderCall(ctx, &request, attempts, callDuration, lastErr)
	if err != nil {
		return nil, NewLLMError(err, ErrCodeLLMGeneration, map[string]any{
			"agent":  request.Agent.ID,
			"action": request.Action.ID,
		})
	}
	return response, nil
}

func (i *llmInvoker) acquireRateLimiter(
	ctx context.Context,
	request *Request,
	backoff *adaptiveBackoff,
	lastErr *error,
) (func(tokens int), error) {
	if request == nil {
		return nil, nil
	}
	limiter := i.cfg.rateLimiter
	if limiter == nil || request.Agent == nil || request.Agent.Model.IsEmpty() {
		return nil, nil
	}
	providerCfg := request.Agent.Model.Config
	providerName := providerCfg.Provider
	if providerName == "" {
		return nil, nil
	}
	if acquireErr := limiter.Acquire(ctx, providerName, providerCfg.RateLimit); acquireErr != nil {
		if lastErr != nil {
			*lastErr = acquireErr
		}
		if isRetryableErrorWithContext(ctx, acquireErr) {
			applyRetryAfterHint(acquireErr, backoff)
			return nil, retry.RetryableError(acquireErr)
		}
		return nil, acquireErr
	}
	return func(tokens int) {
		limiter.Release(ctx, providerName, tokens)
	}, nil
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
		if !request.Agent.Model.IsEmpty() {
			if provider := request.Agent.Model.Config.Provider; provider != "" {
				metadata["provider"] = provider
			}
			if model := request.Agent.Model.Config.Model; model != "" {
				metadata["model"] = model
			}
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

func applyRetryAfterHint(err error, backoff *adaptiveBackoff) {
	if backoff == nil || err == nil {
		return
	}
	if llmErr, ok := llmadapter.IsLLMError(err); ok && llmErr != nil {
		if delay := llmErr.SuggestedRetryDelay(); delay > 0 {
			backoff.setOverride(delay)
		}
	}
}

type adaptiveBackoff struct {
	base     retry.Backoff
	mu       sync.Mutex
	override time.Duration
}

func newAdaptiveBackoff(base retry.Backoff) *adaptiveBackoff {
	return &adaptiveBackoff{base: base}
}

func (a *adaptiveBackoff) Next() (time.Duration, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.override > 0 {
		delay := a.override
		a.override = 0
		if _, stop := a.base.Next(); stop {
			return 0, true
		}
		return delay, false
	}
	return a.base.Next()
}

func (a *adaptiveBackoff) setOverride(delay time.Duration) {
	if a == nil {
		return
	}
	if delay < 0 {
		delay = 0
	}
	a.mu.Lock()
	a.override = delay
	a.mu.Unlock()
}
