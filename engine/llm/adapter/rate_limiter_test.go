package llmadapter

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
	appconfig "github.com/compozy/compozy/pkg/config"
)

type stubProviderMetrics struct {
	mu     sync.Mutex
	delays []time.Duration
}

func (s *stubProviderMetrics) RecordRequest(context.Context, core.ProviderName, string, time.Duration, string) {
}
func (s *stubProviderMetrics) RecordTokens(context.Context, core.ProviderName, string, string, int) {}
func (s *stubProviderMetrics) RecordCost(context.Context, core.ProviderName, string, float64)       {}
func (s *stubProviderMetrics) RecordError(context.Context, core.ProviderName, string, string)       {}
func (s *stubProviderMetrics) RecordRateLimitDelay(_ context.Context, _ core.ProviderName, delay time.Duration) {
	if delay > 0 {
		s.mu.Lock()
		s.delays = append(s.delays, delay)
		s.mu.Unlock()
	}
}

func (s *stubProviderMetrics) Delays() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]time.Duration, len(s.delays))
	copy(out, s.delays)
	return out
}

func TestProviderRateLimiter_ConcurrencyLimit(t *testing.T) {
	t.Parallel()

	registry := NewRateLimiterRegistry(appconfig.LLMRateLimitConfig{
		Enabled:            true,
		DefaultConcurrency: 1,
		DefaultQueueSize:   0,
	}, providermetrics.Nop())

	ctx := context.Background()
	require.NoError(t, registry.Acquire(ctx, core.ProviderOpenAI, nil))

	err := registry.Acquire(ctx, core.ProviderOpenAI, nil)
	require.Error(t, err)
	llmErr, ok := IsLLMError(err)
	require.True(t, ok)
	require.Equal(t, ErrCodeRateLimit, llmErr.Code)

	registry.Release(ctx, core.ProviderOpenAI, 0)
	require.NoError(t, registry.Acquire(ctx, core.ProviderOpenAI, nil))
	registry.Release(ctx, core.ProviderOpenAI, 0)
}

func TestProviderRateLimiter_QueueWaitsForSlot(t *testing.T) {
	t.Parallel()

	registry := NewRateLimiterRegistry(appconfig.LLMRateLimitConfig{
		Enabled:            true,
		DefaultConcurrency: 1,
		DefaultQueueSize:   1,
	}, providermetrics.Nop())

	ctx := context.Background()
	require.NoError(t, registry.Acquire(ctx, core.ProviderOpenAI, nil))

	errCh := make(chan error, 1)
	go func() {
		errCh <- registry.Acquire(ctx, core.ProviderOpenAI, nil)
	}()

	require.Eventually(t, func() bool {
		snapshot, ok := registry.Metrics(core.ProviderOpenAI)
		return ok &&
			snapshot.ActiveRequests == 1 &&
			snapshot.QueuedRequests == 1
	}, time.Second, 10*time.Millisecond)

	registry.Release(ctx, core.ProviderOpenAI, 0)

	require.Eventually(t, func() bool {
		select {
		case err := <-errCh:
			require.NoError(t, err)
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	registry.Release(ctx, core.ProviderOpenAI, 0)
}

func TestProviderRateLimiter_QueueOverflow(t *testing.T) {
	t.Parallel()

	registry := NewRateLimiterRegistry(appconfig.LLMRateLimitConfig{
		Enabled:            true,
		DefaultConcurrency: 1,
		DefaultQueueSize:   1,
	}, providermetrics.Nop())

	ctx := context.Background()
	require.NoError(t, registry.Acquire(ctx, core.ProviderOpenAI, nil))

	waitErr := make(chan error, 1)
	go func() {
		waitErr <- registry.Acquire(ctx, core.ProviderOpenAI, nil)
	}()

	require.Eventually(t, func() bool {
		snapshot, ok := registry.Metrics(core.ProviderOpenAI)
		return ok && snapshot.QueuedRequests == 1
	}, time.Second, 10*time.Millisecond)

	err := registry.Acquire(ctx, core.ProviderOpenAI, nil)
	require.Error(t, err)
	llmErr, ok := IsLLMError(err)
	require.True(t, ok)
	require.Equal(t, ErrCodeRateLimit, llmErr.Code)

	registry.Release(ctx, core.ProviderOpenAI, 0)
	require.NoError(t, <-waitErr)
	registry.Release(ctx, core.ProviderOpenAI, 0)
}

func TestRateLimiterRegistry_Overrides(t *testing.T) {
	t.Parallel()

	registry := NewRateLimiterRegistry(appconfig.LLMRateLimitConfig{
		Enabled:            true,
		DefaultConcurrency: 2,
		DefaultQueueSize:   0,
		PerProviderLimits: map[string]appconfig.ProviderRateLimitConfig{
			"OpenAI": {Concurrency: 3},
		},
	}, providermetrics.Nop())

	override := &core.ProviderRateLimitConfig{Concurrency: 1}
	ctx := context.Background()

	// Override should clamp concurrency to 1.
	require.NoError(t, registry.Acquire(ctx, core.ProviderOpenAI, override))
	err := registry.Acquire(ctx, core.ProviderOpenAI, override)
	require.Error(t, err)
	registry.Release(ctx, core.ProviderOpenAI, 0)

	// Lower-case lookup should also work for defaults.
	require.NoError(t, registry.Acquire(ctx, core.ProviderAnthropic, nil))
	require.NoError(t, registry.Acquire(ctx, core.ProviderAnthropic, nil))
	err = registry.Acquire(ctx, core.ProviderAnthropic, nil)
	require.Error(t, err)
	registry.Release(ctx, core.ProviderAnthropic, 0)
	registry.Release(ctx, core.ProviderAnthropic, 0)
}

func TestProviderRateLimiter_RequestRateLimit(t *testing.T) {
	t.Parallel()

	registry := NewRateLimiterRegistry(appconfig.LLMRateLimitConfig{
		Enabled:            true,
		DefaultConcurrency: 1,
		DefaultQueueSize:   0,
	}, providermetrics.Nop())

	override := &core.ProviderRateLimitConfig{
		Concurrency:       1,
		RequestsPerMinute: 60, // roughly one request per second
	}

	ctx := context.Background()
	require.NoError(t, registry.Acquire(ctx, core.ProviderAnthropic, override))
	registry.Release(ctx, core.ProviderAnthropic, 0)
	start := time.Now()
	require.NoError(t, registry.Acquire(ctx, core.ProviderAnthropic, override))
	elapsed := time.Since(start)
	registry.Release(ctx, core.ProviderAnthropic, 0)

	if elapsed < 800*time.Millisecond {
		t.Fatalf("expected rate limiter to enforce ~1s spacing, elapsed=%v", elapsed)
	}
}

func TestProviderRateLimiter_TokenRateLimit(t *testing.T) {
	t.Parallel()

	registry := NewRateLimiterRegistry(appconfig.LLMRateLimitConfig{
		Enabled:                true,
		DefaultConcurrency:     1,
		DefaultQueueSize:       0,
		DefaultTokensPerMinute: 0,
	}, providermetrics.Nop())

	override := &core.ProviderRateLimitConfig{
		Concurrency:     1,
		TokensPerMinute: 120, // roughly two tokens per second
	}

	ctx := context.Background()
	require.NoError(t, registry.Acquire(ctx, core.ProviderAnthropic, override))
	registry.Release(ctx, core.ProviderAnthropic, 2)
	start := time.Now()
	require.NoError(t, registry.Acquire(ctx, core.ProviderAnthropic, override))
	registry.Release(ctx, core.ProviderAnthropic, 2)
	elapsed := time.Since(start)

	if elapsed < 800*time.Millisecond {
		t.Fatalf("expected token limiter to enforce ~1s spacing, elapsed=%v", elapsed)
	}
}

func TestRateLimiterRegistry_BurstOverrides(t *testing.T) {
	t.Parallel()

	registry := NewRateLimiterRegistry(appconfig.LLMRateLimitConfig{
		Enabled:                  true,
		DefaultConcurrency:       2,
		DefaultQueueSize:         0,
		DefaultRequestsPerMinute: 120,
		DefaultRequestBurst:      7,
	}, providermetrics.Nop())

	ctx := context.Background()
	require.NoError(t, registry.Acquire(ctx, core.ProviderOpenAI, nil))
	registry.Release(ctx, core.ProviderOpenAI, 0)

	value, ok := registry.limiters.Load(strings.ToLower(string(core.ProviderOpenAI)))
	require.True(t, ok)
	limiter, ok := value.(*providerRateLimiter)
	require.True(t, ok)
	require.NotNil(t, limiter.rateLimiter)
	require.Equal(t, 7, limiter.rateLimiter.Burst())

	override := &core.ProviderRateLimitConfig{
		RequestsPerMinute: 90,
		RequestBurst:      5,
	}
	require.NoError(t, registry.Acquire(ctx, core.ProviderAnthropic, override))
	registry.Release(ctx, core.ProviderAnthropic, 0)

	value, ok = registry.limiters.Load(strings.ToLower(string(core.ProviderAnthropic)))
	require.True(t, ok)
	limiter, ok = value.(*providerRateLimiter)
	require.True(t, ok)
	require.NotNil(t, limiter.rateLimiter)
	require.Equal(t, 5, limiter.rateLimiter.Burst())
}

func TestRateLimiterRegistry_RecordsDelay(t *testing.T) {
	t.Parallel()

	metrics := &stubProviderMetrics{}
	registry := NewRateLimiterRegistry(appconfig.LLMRateLimitConfig{
		Enabled:            true,
		DefaultConcurrency: 1,
		DefaultQueueSize:   1,
	}, metrics)

	ctx := context.Background()
	require.NoError(t, registry.Acquire(ctx, core.ProviderOpenAI, nil))

	ready := make(chan struct{})
	go func() {
		require.NoError(t, registry.Acquire(ctx, core.ProviderOpenAI, nil))
		close(ready)
	}()

	require.Eventually(t, func() bool {
		snapshot, ok := registry.Metrics(core.ProviderOpenAI)
		return ok && snapshot.QueuedRequests == 1
	}, time.Second, 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)
	registry.Release(ctx, core.ProviderOpenAI, 0)

	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("queued acquire did not complete")
	}

	require.NotEmpty(t, metrics.Delays())
	require.Greater(t, metrics.Delays()[0], time.Duration(0))
	registry.Release(ctx, core.ProviderOpenAI, 0)
}
