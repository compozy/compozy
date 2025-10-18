package providermetrics

import (
	"context"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	labelProvider   = "provider"
	labelModel      = "model"
	labelOutcome    = "outcome"
	labelTokenType  = "type"
	labelErrorType  = "error_type"
	unitSeconds     = "s"
	unitTokens      = "1"
	unitUSD         = "usd"
	outcomeSuccess  = "success"
	tokenTypePrompt = "prompt"
	tokenTypeOutput = "completion"
)

var (
	requestLatencyBuckets = []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120}
	rateLimitBuckets      = []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60}
)

// Recorder exposes aggregation helpers for provider call metrics.
type Recorder interface {
	RecordRequest(ctx context.Context, provider core.ProviderName, model string, duration time.Duration, outcome string)
	RecordTokens(ctx context.Context, provider core.ProviderName, model string, tokenType string, tokens int)
	RecordCost(ctx context.Context, provider core.ProviderName, model string, costUSD float64)
	RecordError(ctx context.Context, provider core.ProviderName, model string, errorType string)
	RecordRateLimitDelay(ctx context.Context, provider core.ProviderName, delay time.Duration)
}

type otelRecorder struct {
	requestLatency metric.Float64Histogram
	tokensCounter  metric.Int64Counter
	costCounter    metric.Float64Counter
	errorsCounter  metric.Int64Counter
	delayHistogram metric.Float64Histogram
}

type noopRecorder struct{}

// NewRecorder constructs an OpenTelemetry-backed provider metrics recorder.
func NewRecorder(meter metric.Meter) (Recorder, error) {
	if meter == nil {
		return Nop(), nil
	}
	requestLatency, err := meter.Float64Histogram(
		metrics.MetricNameWithSubsystem("llm_provider", "request_seconds"),
		metric.WithDescription("Provider API call latency"),
		metric.WithUnit(unitSeconds),
		metric.WithExplicitBucketBoundaries(requestLatencyBuckets...),
	)
	if err != nil {
		return nil, err
	}
	tokensCounter, err := meter.Int64Counter(
		metrics.MetricNameWithSubsystem("llm_provider", "tokens_total"),
		metric.WithDescription("Total tokens consumed by provider and token type"),
		metric.WithUnit(unitTokens),
	)
	if err != nil {
		return nil, err
	}
	costCounter, err := meter.Float64Counter(
		metrics.MetricNameWithSubsystem("llm_provider", "cost_usd_total"),
		metric.WithDescription("Estimated cumulative cost in USD"),
		metric.WithUnit(unitUSD),
	)
	if err != nil {
		return nil, err
	}
	errorsCounter, err := meter.Int64Counter(
		metrics.MetricNameWithSubsystem("llm_provider", "errors_total"),
		metric.WithDescription("Provider errors grouped by category"),
		metric.WithUnit(unitTokens),
	)
	if err != nil {
		return nil, err
	}
	delayHistogram, err := meter.Float64Histogram(
		metrics.MetricNameWithSubsystem("llm_provider", "rate_limit_delays_seconds"),
		metric.WithDescription("Duration spent waiting due to provider rate limits"),
		metric.WithUnit(unitSeconds),
		metric.WithExplicitBucketBoundaries(rateLimitBuckets...),
	)
	if err != nil {
		return nil, err
	}
	return &otelRecorder{
		requestLatency: requestLatency,
		tokensCounter:  tokensCounter,
		costCounter:    costCounter,
		errorsCounter:  errorsCounter,
		delayHistogram: delayHistogram,
	}, nil
}

// Nop returns a recorder that ignores all invocations.
func Nop() Recorder { return noopRecorder{} }

func (r *otelRecorder) RecordRequest(
	ctx context.Context,
	provider core.ProviderName,
	model string,
	duration time.Duration,
	outcome string,
) {
	if r == nil || r.requestLatency == nil {
		return
	}
	attrs := providerModelAttrs(provider, model)
	outcomeAttr := attribute.String(labelOutcome, sanitizeEmpty(outcome, outcomeSuccess))
	attrs = append(attrs, outcomeAttr)
	r.requestLatency.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

func (r *otelRecorder) RecordTokens(
	ctx context.Context,
	provider core.ProviderName,
	model string,
	tokenType string,
	tokens int,
) {
	if r == nil || r.tokensCounter == nil || tokens <= 0 {
		return
	}
	attrs := providerModelAttrs(provider, model)
	tokenAttr := attribute.String(labelTokenType, sanitizeEmpty(tokenType, tokenTypePrompt))
	attrs = append(attrs, tokenAttr)
	r.tokensCounter.Add(ctx, int64(tokens), metric.WithAttributes(attrs...))
}

func (r *otelRecorder) RecordCost(
	ctx context.Context,
	provider core.ProviderName,
	model string,
	costUSD float64,
) {
	if r == nil || r.costCounter == nil || costUSD <= 0 {
		return
	}
	attrs := providerModelAttrs(provider, model)
	r.costCounter.Add(ctx, costUSD, metric.WithAttributes(attrs...))
}

func (r *otelRecorder) RecordError(
	ctx context.Context,
	provider core.ProviderName,
	model string,
	errorType string,
) {
	if r == nil || r.errorsCounter == nil || strings.TrimSpace(errorType) == "" {
		return
	}
	attrs := providerModelAttrs(provider, model)
	attrs = append(attrs, attribute.String(labelErrorType, strings.TrimSpace(errorType)))
	r.errorsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func (r *otelRecorder) RecordRateLimitDelay(
	ctx context.Context,
	provider core.ProviderName,
	delay time.Duration,
) {
	if r == nil || r.delayHistogram == nil || delay <= 0 {
		return
	}
	attrs := []attribute.KeyValue{attribute.String(labelProvider, normalizeProvider(provider))}
	r.delayHistogram.Record(ctx, delay.Seconds(), metric.WithAttributes(attrs...))
}

func (noopRecorder) RecordRequest(context.Context, core.ProviderName, string, time.Duration, string) {
}
func (noopRecorder) RecordTokens(context.Context, core.ProviderName, string, string, int)   {}
func (noopRecorder) RecordCost(context.Context, core.ProviderName, string, float64)         {}
func (noopRecorder) RecordError(context.Context, core.ProviderName, string, string)         {}
func (noopRecorder) RecordRateLimitDelay(context.Context, core.ProviderName, time.Duration) {}

func providerModelAttrs(provider core.ProviderName, model string) []attribute.KeyValue {
	name := normalizeProvider(provider)
	modelName := strings.TrimSpace(model)
	if modelName == "" {
		modelName = "unknown"
	}
	return []attribute.KeyValue{
		attribute.String(labelProvider, name),
		attribute.String(labelModel, modelName),
	}
}

func normalizeProvider(provider core.ProviderName) string {
	name := strings.TrimSpace(string(provider))
	if name == "" {
		return "unknown"
	}
	return strings.ToLower(name)
}

func sanitizeEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
