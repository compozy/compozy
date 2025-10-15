package monitoring

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm/usage"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// #nosec G101 -- metric identifiers are safe constants
const (
	llmPromptTokensMetric     = "compozy_llm_prompt_tokens_total"
	llmCompletionTokensMetric = "compozy_llm_completion_tokens_total"
	llmUsageEventsMetric      = "compozy_llm_usage_events_total"
	llmUsageFailuresMetric    = "compozy_llm_usage_failures_total"
	llmUsageLatencyMetric     = "compozy_llm_usage_latency_seconds"

	labelComponent = "component"
	labelProvider  = "provider"
	labelModel     = "model"
	labelOutcome   = "outcome"

	outcomeSuccess = "success"
	outcomeFailure = "failure"
)

var llmLatencyBuckets = []float64{
	0.001,
	0.005,
	0.01,
	0.025,
	0.05,
	0.1,
	0.25,
	0.5,
	1,
	2.5,
	5,
	10,
}

type llmUsageMetrics struct {
	promptTokens     metric.Int64Counter
	completionTokens metric.Int64Counter
	events           metric.Int64Counter
	failures         metric.Int64Counter
	latency          metric.Float64Histogram
}

func newLLMUsageMetrics(meter metric.Meter) (usage.Metrics, error) {
	if meter == nil {
		return &llmUsageMetrics{}, nil
	}
	promptTokens, err := meter.Int64Counter(
		llmPromptTokensMetric,
		metric.WithDescription("Total prompt tokens recorded for LLM executions"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	completionTokens, err := meter.Int64Counter(
		llmCompletionTokensMetric,
		metric.WithDescription("Total completion tokens recorded for LLM executions"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	events, err := meter.Int64Counter(
		llmUsageEventsMetric,
		metric.WithDescription("Total LLM usage events processed by collectors"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	failures, err := meter.Int64Counter(
		llmUsageFailuresMetric,
		metric.WithDescription("Total LLM usage persistence failures"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	latency, err := meter.Float64Histogram(
		llmUsageLatencyMetric,
		metric.WithDescription("Histogram of LLM usage persistence latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(llmLatencyBuckets...),
	)
	if err != nil {
		return nil, err
	}
	return &llmUsageMetrics{
		promptTokens:     promptTokens,
		completionTokens: completionTokens,
		events:           events,
		failures:         failures,
		latency:          latency,
	}, nil
}

func (m *llmUsageMetrics) RecordSuccess(
	ctx context.Context,
	component core.ComponentType,
	provider string,
	model string,
	promptTokens int,
	completionTokens int,
	latency time.Duration,
) {
	if m == nil {
		return
	}
	attrs := usageAttributes(component, provider, model)
	if m.promptTokens != nil && promptTokens != 0 {
		m.promptTokens.Add(ctx, int64(promptTokens), metric.WithAttributes(attrs...))
	}
	if m.completionTokens != nil && completionTokens != 0 {
		m.completionTokens.Add(ctx, int64(completionTokens), metric.WithAttributes(attrs...))
	}
	outcomeAttrs := make([]attribute.KeyValue, 0, len(attrs)+1)
	outcomeAttrs = append(outcomeAttrs, attrs...)
	outcomeAttrs = append(outcomeAttrs, attribute.String(labelOutcome, outcomeSuccess))
	if m.events != nil {
		m.events.Add(ctx, 1, metric.WithAttributes(outcomeAttrs...))
	}
	if m.latency != nil {
		m.latency.Record(ctx, latency.Seconds(), metric.WithAttributes(outcomeAttrs...))
	}
}

func (m *llmUsageMetrics) RecordFailure(
	ctx context.Context,
	component core.ComponentType,
	provider string,
	model string,
	latency time.Duration,
) {
	if m == nil {
		return
	}
	attrs := usageAttributes(component, provider, model)
	outcomeAttrs := make([]attribute.KeyValue, 0, len(attrs)+1)
	outcomeAttrs = append(outcomeAttrs, attrs...)
	outcomeAttrs = append(outcomeAttrs, attribute.String(labelOutcome, outcomeFailure))
	if m.failures != nil {
		m.failures.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	if m.events != nil {
		m.events.Add(ctx, 1, metric.WithAttributes(outcomeAttrs...))
	}
	if m.latency != nil {
		m.latency.Record(ctx, latency.Seconds(), metric.WithAttributes(outcomeAttrs...))
	}
}

func usageAttributes(component core.ComponentType, provider, model string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(labelComponent, string(component)),
		attribute.String(labelProvider, normalizeLabelValue(provider)),
		attribute.String(labelModel, normalizeLabelValue(model)),
	}
}

func normalizeLabelValue(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
