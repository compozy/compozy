package llmadapter

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"

	"github.com/compozy/compozy/engine/core"
	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
	appconfig "github.com/compozy/compozy/pkg/config"
)

const rateLimitFloatEpsilon = 1e-9

// RateLimiterRegistry coordinates provider-specific concurrency throttles backed by semaphores.
// The registry shares limiters across orchestrator runs so parallel workflows observe the same limits.
type RateLimiterRegistry struct {
	enabled bool
	config  appconfig.LLMRateLimitConfig

	limiters sync.Map // map[string]*providerRateLimiter (lower-cased provider names)
	metrics  providermetrics.Recorder
}

// RateLimiterMetricsSnapshot provides introspection for active/queued/rejected counters.
type RateLimiterMetricsSnapshot struct {
	ActiveRequests   int32
	QueuedRequests   int32
	RejectedRequests int64
	TotalRequests    int64
}

type providerRateLimiter struct {
	provider core.ProviderName
	enabled  bool

	concurrency       int64
	queueSize         int64
	requestsPerMinute float64
	tokensPerMinute   float64
	requestBurst      int
	tokenBurst        int

	sem          *semaphore.Weighted
	queueSem     *semaphore.Weighted
	rateLimiter  *rate.Limiter
	tokenLimiter *rate.Limiter

	metrics limiterMetrics
}

type limiterMetrics struct {
	activeRequests   atomic.Int32
	queuedRequests   atomic.Int32
	rejectedRequests atomic.Int64
	totalRequests    atomic.Int64
}

type limiterSettings struct {
	concurrency       int64
	queueSize         int64
	requestsPerMinute float64
	tokensPerMinute   float64
	requestBurst      int
	tokenBurst        int
}

// NewRateLimiterRegistry creates a registry using the supplied configuration.
func NewRateLimiterRegistry(cfg appconfig.LLMRateLimitConfig, recorder providermetrics.Recorder) *RateLimiterRegistry {
	cfg.PerProviderLimits = normalizeProviderLimits(cfg.PerProviderLimits)
	if recorder == nil {
		recorder = providermetrics.Nop()
	}
	return &RateLimiterRegistry{
		enabled: cfg.Enabled,
		config:  cfg,
		metrics: recorder,
	}
}

// SetRecorder replaces the metrics recorder used by the registry and its children.
func (r *RateLimiterRegistry) SetRecorder(rec providermetrics.Recorder) {
	if rec == nil {
		rec = providermetrics.Nop()
	}
	r.metrics = rec
}

// Acquire reserves a concurrency slot for the requested provider.
// When rate limiting is disabled or resolves to zero concurrency, Acquire is a no-op.
func (r *RateLimiterRegistry) Acquire(
	ctx context.Context,
	provider core.ProviderName,
	override *core.ProviderRateLimitConfig,
) error {
	if provider == "" {
		return nil
	}
	limiter := r.ensureLimiter(provider, override)
	if limiter == nil {
		return nil
	}
	start := time.Now()
	err := limiter.acquire(ctx)
	if delay := time.Since(start); delay > 0 && r.metrics != nil {
		r.metrics.RecordRateLimitDelay(ctx, provider, delay)
	}
	return err
}

// Release frees a previously acquired slot for the requested provider.
func (r *RateLimiterRegistry) Release(ctx context.Context, provider core.ProviderName, tokens int) {
	if provider == "" {
		return
	}
	key := strings.ToLower(string(provider))
	value, ok := r.limiters.Load(key)
	if !ok {
		return
	}
	if limiter, ok := value.(*providerRateLimiter); ok {
		limiter.release(ctx, tokens)
	}
}

// Metrics provides a snapshot of limiter counters for observability and tests.
func (r *RateLimiterRegistry) Metrics(provider core.ProviderName) (RateLimiterMetricsSnapshot, bool) {
	if provider == "" {
		return RateLimiterMetricsSnapshot{}, false
	}
	key := strings.ToLower(string(provider))
	value, ok := r.limiters.Load(key)
	if !ok {
		return RateLimiterMetricsSnapshot{}, false
	}
	if limiter, ok := value.(*providerRateLimiter); ok {
		return limiter.metrics.snapshot(), true
	}
	return RateLimiterMetricsSnapshot{}, false
}

func (m *limiterMetrics) snapshot() RateLimiterMetricsSnapshot {
	if m == nil {
		return RateLimiterMetricsSnapshot{}
	}
	return RateLimiterMetricsSnapshot{
		ActiveRequests:   m.activeRequests.Load(),
		QueuedRequests:   m.queuedRequests.Load(),
		RejectedRequests: m.rejectedRequests.Load(),
		TotalRequests:    m.totalRequests.Load(),
	}
}

func (r *RateLimiterRegistry) ensureLimiter(
	provider core.ProviderName,
	override *core.ProviderRateLimitConfig,
) *providerRateLimiter {
	if !r.enabled {
		return nil
	}
	settings := r.buildSettings(provider, override)
	if settings.concurrency <= 0 {
		return nil
	}
	key := strings.ToLower(string(provider))
	if existing, ok := r.limiters.Load(key); ok {
		if current, ok := existing.(*providerRateLimiter); ok {
			if current.matches(settings) {
				return current
			}
		}
		newLimiter := newProviderRateLimiter(provider, settings)
		r.limiters.Store(key, newLimiter)
		return newLimiter
	}
	limiter := newProviderRateLimiter(provider, settings)
	actual, loaded := r.limiters.LoadOrStore(key, limiter)
	if loaded {
		if existing, ok := actual.(*providerRateLimiter); ok {
			return existing
		}
		return limiter
	}
	return limiter
}

func (r *RateLimiterRegistry) buildSettings(
	provider core.ProviderName,
	override *core.ProviderRateLimitConfig,
) limiterSettings {
	settings := r.defaultLimiterSettings()
	settings = r.applyPerProviderSettings(settings, provider)
	return applyOverrideLimiterSettings(settings, override)
}

// defaultLimiterSettings creates base limiter settings from global defaults.
func (r *RateLimiterRegistry) defaultLimiterSettings() limiterSettings {
	return limiterSettings{
		concurrency:       int64(r.config.DefaultConcurrency),
		queueSize:         int64(r.config.DefaultQueueSize),
		requestsPerMinute: float64(r.config.DefaultRequestsPerMinute),
		tokensPerMinute:   float64(r.config.DefaultTokensPerMinute),
		requestBurst:      r.config.DefaultRequestBurst,
		tokenBurst:        r.config.DefaultTokenBurst,
	}
}

// applyPerProviderSettings merges provider-specific config overrides into the limiter settings.
func (r *RateLimiterRegistry) applyPerProviderSettings(
	settings limiterSettings,
	provider core.ProviderName,
) limiterSettings {
	perProvider := r.lookupPerProvider(strings.ToLower(string(provider)))
	if perProvider == nil {
		return settings
	}
	if perProvider.Concurrency > 0 {
		settings.concurrency = int64(perProvider.Concurrency)
	}
	if perProvider.QueueSize > 0 {
		settings.queueSize = int64(perProvider.QueueSize)
	}
	if perProvider.RequestsPerMinute > 0 {
		settings.requestsPerMinute = float64(perProvider.RequestsPerMinute)
	}
	if perProvider.TokensPerMinute > 0 {
		settings.tokensPerMinute = float64(perProvider.TokensPerMinute)
	}
	if perProvider.RequestBurst > 0 {
		settings.requestBurst = perProvider.RequestBurst
	}
	if perProvider.TokenBurst > 0 {
		settings.tokenBurst = perProvider.TokenBurst
	}
	return settings
}

// applyOverrideLimiterSettings merges runtime overrides into limiter settings.
func applyOverrideLimiterSettings(settings limiterSettings, override *core.ProviderRateLimitConfig) limiterSettings {
	if override == nil {
		return settings
	}
	if override.Concurrency > 0 {
		settings.concurrency = int64(override.Concurrency)
	}
	if override.QueueSize > 0 {
		settings.queueSize = int64(override.QueueSize)
	}
	if override.RequestsPerMinute > 0 {
		settings.requestsPerMinute = float64(override.RequestsPerMinute)
	}
	if override.TokensPerMinute > 0 {
		settings.tokensPerMinute = float64(override.TokensPerMinute)
	}
	if override.RequestBurst > 0 {
		settings.requestBurst = override.RequestBurst
	}
	if override.TokenBurst > 0 {
		settings.tokenBurst = override.TokenBurst
	}
	return settings
}

func (r *RateLimiterRegistry) lookupPerProvider(
	provider string,
) *appconfig.ProviderRateLimitConfig {
	if len(r.config.PerProviderLimits) == 0 {
		return nil
	}
	if cfg, ok := r.config.PerProviderLimits[provider]; ok {
		return &cfg
	}
	return nil
}

func newProviderRateLimiter(
	provider core.ProviderName,
	settings limiterSettings,
) *providerRateLimiter {
	limiter := &providerRateLimiter{
		provider:          provider,
		enabled:           settings.concurrency > 0,
		concurrency:       settings.concurrency,
		queueSize:         settings.queueSize,
		requestsPerMinute: settings.requestsPerMinute,
		tokensPerMinute:   settings.tokensPerMinute,
		requestBurst:      settings.requestBurst,
		tokenBurst:        settings.tokenBurst,
	}
	if !limiter.enabled {
		return limiter
	}
	limiter.sem = semaphore.NewWeighted(settings.concurrency)
	if settings.queueSize > 0 {
		limiter.queueSem = semaphore.NewWeighted(settings.queueSize)
	}
	if settings.requestsPerMinute > 0 {
		perSecond := settings.requestsPerMinute / 60.0
		burst := computeBurst(perSecond, settings.requestBurst)
		limiter.rateLimiter = rate.NewLimiter(rate.Limit(perSecond), burst)
	}
	if settings.tokensPerMinute > 0 {
		perSecond := settings.tokensPerMinute / 60.0
		burst := computeBurst(perSecond, settings.tokenBurst)
		limiter.tokenLimiter = rate.NewLimiter(rate.Limit(perSecond), burst)
	}
	return limiter
}

func computeBurst(perSecond float64, configured int) int {
	if configured > 0 {
		return configured
	}
	if perSecond <= 0 {
		return 1
	}
	return int(math.Ceil(perSecond))
}

func (l *providerRateLimiter) matches(settings limiterSettings) bool {
	if l == nil {
		return false
	}
	return l.concurrency == settings.concurrency &&
		l.queueSize == settings.queueSize &&
		math.Abs(l.requestsPerMinute-settings.requestsPerMinute) < rateLimitFloatEpsilon &&
		math.Abs(l.tokensPerMinute-settings.tokensPerMinute) < rateLimitFloatEpsilon &&
		l.requestBurst == settings.requestBurst &&
		l.tokenBurst == settings.tokenBurst
}

func (l *providerRateLimiter) acquire(ctx context.Context) error {
	if l == nil || !l.enabled || l.sem == nil {
		return nil
	}
	l.metrics.totalRequests.Add(1)

	if l.sem.TryAcquire(1) {
		l.metrics.activeRequests.Add(1)
		if l.rateLimiter != nil {
			if err := l.rateLimiter.Wait(ctx); err != nil {
				l.metrics.rejectedRequests.Add(1)
				l.sem.Release(1)
				l.metrics.activeRequests.Add(-1)
				return newRateLimitError(l.provider, "provider request rate wait canceled", err)
			}
		}
		return nil
	}

	if l.queueSem == nil {
		l.metrics.rejectedRequests.Add(1)
		return newRateLimitError(l.provider, "provider concurrency limit reached", nil)
	}

	if !l.queueSem.TryAcquire(1) {
		l.metrics.rejectedRequests.Add(1)
		return newRateLimitError(l.provider, "provider rate limit queue is full", nil)
	}
	l.metrics.queuedRequests.Add(1)
	defer func() {
		l.queueSem.Release(1)
		l.metrics.queuedRequests.Add(-1)
	}()

	if err := l.sem.Acquire(ctx, 1); err != nil {
		l.metrics.rejectedRequests.Add(1)
		return newRateLimitError(l.provider, "provider rate limit wait canceled", err)
	}
	l.metrics.activeRequests.Add(1)
	if l.rateLimiter != nil {
		if err := l.rateLimiter.Wait(ctx); err != nil {
			l.metrics.rejectedRequests.Add(1)
			l.sem.Release(1)
			l.metrics.activeRequests.Add(-1)
			return newRateLimitError(l.provider, "provider request rate wait canceled", err)
		}
	}
	return nil
}

func (l *providerRateLimiter) release(ctx context.Context, tokens int) {
	if l == nil || !l.enabled || l.sem == nil {
		return
	}
	if l.tokenLimiter != nil && tokens > 0 {
		if ctx != nil {
			_ = l.tokenLimiter.WaitN( //nolint:errcheck // release is best-effort; limiter handles context cancellation
				context.WithoutCancel(ctx),
				tokens,
			)
		}
	}
	l.sem.Release(1)
	l.metrics.activeRequests.Add(-1)
}

func newRateLimitError(provider core.ProviderName, message string, underlying error) error {
	details := message
	if provider != "" {
		details = fmt.Sprintf("%s (%s)", message, provider)
	}
	return NewErrorWithCode(ErrCodeRateLimit, details, string(provider), underlying)
}

func normalizeProviderLimits(
	input map[string]appconfig.ProviderRateLimitConfig,
) map[string]appconfig.ProviderRateLimitConfig {
	if len(input) == 0 {
		return nil
	}
	normalized := make(map[string]appconfig.ProviderRateLimitConfig, len(input))
	for key, value := range input {
		canonical := strings.ToLower(strings.TrimSpace(key))
		if canonical == "" {
			continue
		}
		normalized[canonical] = value
	}
	return normalized
}
