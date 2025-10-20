package orchestrator

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/provider/pricing"
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

const (
	providerOutcomeSuccess     = "success"
	providerOutcomeError       = "error"
	providerOutcomeTimeout     = "timeout"
	providerOutcomeRateLimited = "rate_limited"

	errorTypeAuth       = "auth"
	errorTypeRateLimit  = "rate_limit"
	errorTypeInvalid    = "invalid_request"
	errorTypeServer     = "server_error"
	errorTypeTimeout    = "timeout"
	errorTypeUnknown    = "unknown"
	tokenTypePrompt     = "prompt"
	tokenTypeCompletion = "completion"
)

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
		jitterMax := i.cfg.retryJitterMax
		if jitterMax <= 0 {
			jitterMax = defaultRetryJitter
		}
		return retry.WithMaxRetries(maxRetries, retry.WithJitter(jitterMax, exponential))
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
	providerName, modelName := providerAndModel(request)
	releaseLimiter, acquireErr := i.acquireRateLimiter(ctx, request, backoff, lastErr)
	if acquireErr != nil {
		i.recordAttemptMetrics(ctx, providerName, modelName, 0, acquireErr)
		return acquireErr
	}
	tokensUsed := 0
	if releaseLimiter != nil {
		defer func() {
			releaseLimiter(tokensUsed)
		}()
	}
	start := time.Now()
	resp, callErr := client.GenerateContent(ctx, req)
	duration := time.Since(start)
	i.recordAttemptMetrics(ctx, providerName, modelName, duration, callErr)
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
	if err == nil {
		i.recordUsageMetrics(ctx, &request, response)
	}
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
	if limiter == nil || request.Agent == nil {
		return nil, nil
	}
	model := request.Agent.Model
	if model.IsEmpty() || !model.HasConfig() {
		return nil, nil
	}
	providerCfg := model.Config
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
	outcome, errorType := classifyProviderError(err)
	metadata := map[string]any{
		"latency_ms": float64(duration) / float64(time.Millisecond),
		"attempts":   attempts,
		"retry_cap":  i.cfg.retryAttempts,
		"outcome":    outcome,
	}
	if i.cfg.timeout > 0 {
		metadata["timeout_ms"] = float64(i.cfg.timeout) / float64(time.Millisecond)
	}
	if request != nil && request.Agent != nil {
		metadata["agent_id"] = request.Agent.ID
		model := request.Agent.Model
		if !model.IsEmpty() && model.HasConfig() {
			if provider := model.Config.Provider; provider != "" {
				metadata["provider"] = provider
			}
			if modelID := model.Config.Model; modelID != "" {
				metadata["model"] = modelID
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
		if errorType != "" && errorType != errorTypeUnknown {
			event.Metadata["error_type"] = errorType
		}
	}
	telemetry.RecordEvent(ctx, &event)
}

func (i *llmInvoker) recordAttemptMetrics(
	ctx context.Context,
	provider core.ProviderName,
	model string,
	duration time.Duration,
	callErr error,
) {
	recorder := i.cfg.providerMetrics
	if recorder == nil {
		return
	}
	outcome, errorType := classifyProviderError(callErr)
	recorder.RecordRequest(ctx, provider, model, duration, outcome)
	if outcome != providerOutcomeSuccess && errorType != "" && errorType != errorTypeUnknown {
		recorder.RecordError(ctx, provider, model, errorType)
	}
}

func (i *llmInvoker) recordUsageMetrics(
	ctx context.Context,
	request *Request,
	response *llmadapter.LLMResponse,
) {
	if response == nil || response.Usage == nil {
		return
	}
	recorder := i.cfg.providerMetrics
	if recorder == nil {
		return
	}
	provider, model := providerAndModel(request)
	usage := response.Usage
	if usage.PromptTokens > 0 {
		recorder.RecordTokens(ctx, provider, model, tokenTypePrompt, usage.PromptTokens)
	}
	if usage.CompletionTokens > 0 {
		recorder.RecordTokens(ctx, provider, model, tokenTypeCompletion, usage.CompletionTokens)
	}
	if cost, ok := pricing.EstimateCostUSD(provider, model, usage); ok {
		recorder.RecordCost(ctx, provider, model, cost)
	}
}

func providerAndModel(request *Request) (core.ProviderName, string) {
	if request == nil || request.Agent == nil {
		return "", ""
	}
	model := request.Agent.Model
	if model.IsEmpty() || !model.HasConfig() {
		return "", ""
	}
	return model.Config.Provider, model.Config.Model
}

func classifyProviderError(err error) (string, string) {
	if err == nil {
		return providerOutcomeSuccess, ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return providerOutcomeTimeout, errorTypeTimeout
	}
	if llmErr, ok := llmadapter.IsLLMError(err); ok && llmErr != nil {
		switch llmErr.Code {
		case llmadapter.ErrCodeRateLimit, llmadapter.ErrCodeQuotaExceeded:
			return providerOutcomeRateLimited, errorTypeRateLimit
		case llmadapter.ErrCodeTimeout, llmadapter.ErrCodeGatewayTimeout:
			return providerOutcomeTimeout, errorTypeTimeout
		case llmadapter.ErrCodeUnauthorized, llmadapter.ErrCodeForbidden:
			return providerOutcomeError, errorTypeAuth
		case llmadapter.ErrCodeBadRequest, llmadapter.ErrCodeInvalidModel, llmadapter.ErrCodeContentPolicy:
			return providerOutcomeError, errorTypeInvalid
		case llmadapter.ErrCodeInternalServer,
			llmadapter.ErrCodeBadGateway,
			llmadapter.ErrCodeServiceUnavailable,
			llmadapter.ErrCodeCapacityError,
			llmadapter.ErrCodeConnectionReset,
			llmadapter.ErrCodeConnectionRefused:
			return providerOutcomeError, errorTypeServer
		}
		if llmErr.HTTPStatus >= 500 {
			return providerOutcomeError, errorTypeServer
		}
		if llmErr.HTTPStatus >= 400 {
			return providerOutcomeError, errorTypeInvalid
		}
		return providerOutcomeError, errorTypeUnknown
	}
	return providerOutcomeError, errorTypeUnknown
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
