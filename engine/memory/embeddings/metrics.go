package embeddings

import (
	"context"
	"fmt"
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
)

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
	attrs := []attribute.KeyValue{
		attribute.String(labelProvider, provider),
		attribute.String(labelModel, model),
		attribute.Int(labelBatchSize, batchSize),
	}
	metricInstruments.generationLatency.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	if tokenCount > 0 {
		metricInstruments.tokensTotal.Add(ctx, int64(tokenCount), metric.WithAttributes(
			attribute.String(labelProvider, provider),
			attribute.String(labelModel, model),
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
		attribute.String(labelModel, model),
		attribute.String(labelErrorType, string(errorType)),
	))
}

func ensureInstruments(ctx context.Context) bool {
	metricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter(meterName)
		latency, err := meter.Float64Histogram(
			monitoringmetrics.MetricNameWithSubsystem(subsystemEmbeddings, "generate_seconds"),
			metric.WithDescription("Embedding generation latency"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5),
		)
		if err != nil {
			metricsInitErr = fmt.Errorf("create embeddings latency histogram: %w", err)
			return
		}
		tokens, err := meter.Int64Counter(
			monitoringmetrics.MetricNameWithSubsystem(subsystemEmbeddings, "tokens_total"),
			metric.WithDescription("Total tokens processed for embeddings"),
			metric.WithUnit("1"),
		)
		if err != nil {
			metricsInitErr = fmt.Errorf("create embeddings tokens counter: %w", err)
			return
		}
		hits, err := meter.Int64Counter(
			monitoringmetrics.MetricNameWithSubsystem(subsystemEmbeddings, "cache_hits_total"),
			metric.WithDescription("Embedding cache hits"),
			metric.WithUnit("1"),
		)
		if err != nil {
			metricsInitErr = fmt.Errorf("create embeddings cache hits counter: %w", err)
			return
		}
		misses, err := meter.Int64Counter(
			monitoringmetrics.MetricNameWithSubsystem(subsystemEmbeddings, "cache_misses_total"),
			metric.WithDescription("Embedding cache misses"),
			metric.WithUnit("1"),
		)
		if err != nil {
			metricsInitErr = fmt.Errorf("create embeddings cache misses counter: %w", err)
			return
		}
		errorsCounter, err := meter.Int64Counter(
			monitoringmetrics.MetricNameWithSubsystem(subsystemEmbeddings, "errors_total"),
			metric.WithDescription("Embedding generation errors"),
			metric.WithUnit("1"),
		)
		if err != nil {
			metricsInitErr = fmt.Errorf("create embeddings errors counter: %w", err)
			return
		}
		metricInstruments = instruments{
			generationLatency: latency,
			tokensTotal:       tokens,
			cacheHitsTotal:    hits,
			cacheMissesTotal:  misses,
			errorsTotal:       errorsCounter,
		}
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
func resetMetrics() {
	metricsOnce = sync.Once{}
	errorLogOnce = sync.Once{}
	metricsInitErr = nil
	metricInstruments = instruments{}
}
