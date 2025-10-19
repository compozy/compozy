package embeddings

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	meterName           = "compozy.memory.embeddings"
	subsystemEmbeddings = "memory_embeddings"
	labelProvider       = "provider"
	labelModel          = "model"
	labelBatchSize      = "batch_size"
	labelErrorType      = "error_type"
	modelOther          = "other"
)

var defaultLatencyBuckets = []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5}

var (
	metricsOnce       sync.Once
	metricsInitErr    error
	errorLogOnce      sync.Once
	metricInstruments instruments
)

// ErrorType enumerates embedding error categories tracked in metrics.
type ErrorType string

const (
	// ErrorTypeAuth represents authentication or authorization failures.
	ErrorTypeAuth ErrorType = "auth"
	// ErrorTypeRateLimit represents rate limiting responses from the provider.
	ErrorTypeRateLimit ErrorType = "rate_limit"
	// ErrorTypeInvalidInput represents client-side validation or input errors.
	ErrorTypeInvalidInput ErrorType = "invalid_input"
	// ErrorTypeServerError represents provider-side internal errors.
	ErrorTypeServerError ErrorType = "server_error"
)

// normalizeModelName reduces model cardinality by mapping known model patterns to stable names.
func normalizeModelName(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return modelOther
	}
	// Map known prefixes to stable names to prevent cardinality explosion
	// TODO: Expand this whitelist as new stable models are identified
	switch {
	case strings.HasPrefix(normalized, "text-embedding-ada"):
		return "text-embedding-ada"
	case strings.HasPrefix(normalized, "text-embedding-3"):
		return "text-embedding-3"
	case strings.HasPrefix(normalized, "embed-"):
		return "embed-generic"
	case strings.HasPrefix(normalized, "voyage-"):
		return "voyage-generic"
	default:
		return modelOther
	}
}

type instruments struct {
	generationLatency metric.Float64Histogram
	tokensTotal       metric.Int64Counter
	cacheHitsTotal    metric.Int64Counter
	cacheMissesTotal  metric.Int64Counter
	errorsTotal       metric.Int64Counter
}

// RecordGeneration captures latency and token usage for embedding generation requests.
func RecordGeneration(
	ctx context.Context,
	provider string,
	model string,
	batchSize int,
	duration time.Duration,
	tokenCount int,
) {
	if !ensureInstruments(ctx) {
		return
	}
	normalizedModel := normalizeModelName(model)
	attrs := []attribute.KeyValue{
		attribute.String(labelProvider, provider),
		attribute.String(labelModel, normalizedModel),
		attribute.Int(labelBatchSize, batchSize),
	}
	metricInstruments.generationLatency.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	if tokenCount > 0 {
		metricInstruments.tokensTotal.Add(ctx, int64(tokenCount), metric.WithAttributes(
			attribute.String(labelProvider, provider),
			attribute.String(labelModel, normalizedModel),
		))
	}
}

// RecordCacheHit increments the counter for embedding cache hits.
func RecordCacheHit(ctx context.Context, provider string) {
	if !ensureInstruments(ctx) {
		return
	}
	metricInstruments.cacheHitsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String(labelProvider, provider)))
}

// RecordCacheMiss increments the counter for embedding cache misses.
func RecordCacheMiss(ctx context.Context, provider string) {
	if !ensureInstruments(ctx) {
		return
	}
	metricInstruments.cacheMissesTotal.Add(ctx, 1, metric.WithAttributes(attribute.String(labelProvider, provider)))
}

// RecordError increments the counter for embedding errors grouped by type.
func RecordError(ctx context.Context, provider string, model string, errorType ErrorType) {
	if !ensureInstruments(ctx) {
		return
	}
	metricInstruments.errorsTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String(labelProvider, provider),
		attribute.String(labelModel, normalizeModelName(model)),
		attribute.String(labelErrorType, string(errorType)),
	))
}

func newInstruments(meter metric.Meter) (instruments, error) {
	latency, err := meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem(subsystemEmbeddings, "generate_seconds"),
		metric.WithDescription("Embedding generation latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(defaultLatencyBuckets...),
	)
	if err != nil {
		return instruments{}, fmt.Errorf("create embeddings latency histogram: %w", err)
	}
	tokens, err := meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem(subsystemEmbeddings, "tokens_total"),
		metric.WithDescription("Total tokens processed for embeddings"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return instruments{}, fmt.Errorf("create embeddings tokens counter: %w", err)
	}
	hits, err := meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem(subsystemEmbeddings, "cache_hits_total"),
		metric.WithDescription("Embedding cache hits"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return instruments{}, fmt.Errorf("create embeddings cache hits counter: %w", err)
	}
	misses, err := meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem(subsystemEmbeddings, "cache_misses_total"),
		metric.WithDescription("Embedding cache misses"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return instruments{}, fmt.Errorf("create embeddings cache misses counter: %w", err)
	}
	errorsCounter, err := meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem(subsystemEmbeddings, "errors_total"),
		metric.WithDescription("Embedding generation errors"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return instruments{}, fmt.Errorf("create embeddings errors counter: %w", err)
	}
	return instruments{
		generationLatency: latency,
		tokensTotal:       tokens,
		cacheHitsTotal:    hits,
		cacheMissesTotal:  misses,
		errorsTotal:       errorsCounter,
	}, nil
}

func ensureInstruments(ctx context.Context) bool {
	metricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter(meterName)
		ins, err := newInstruments(meter)
		if err != nil {
			metricsInitErr = err
			return
		}
		metricInstruments = ins
	})
	if metricsInitErr != nil {
		errorLogOnce.Do(func() {
			logger.FromContext(ctx).Error("embedding metrics disabled", "error", metricsInitErr)
		})
		return false
	}
	return true
}

// resetMetrics is intended for tests.
//

func resetMetrics() {
	metricsOnce = sync.Once{}
	errorLogOnce = sync.Once{}
	metricsInitErr = nil
	metricInstruments = instruments{}
}
