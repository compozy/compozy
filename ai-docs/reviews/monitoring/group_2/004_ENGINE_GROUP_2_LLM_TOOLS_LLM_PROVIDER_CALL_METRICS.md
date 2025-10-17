---
title: "LLM Provider Call Metrics"
group: "ENGINE_GROUP_2_LLM_TOOLS"
category: "monitoring"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_2_LLM_TOOLS_MONITORING.md"
issue_index: "1"
sequence: "4"
---

## LLM Provider Call Metrics

**Priority:** 🔴 CRITICAL

**Location:** `engine/llm/provider/`

**Why Critical:**

- Provider calls are most expensive operations (cost & latency)
- No visibility into token usage, rate limits, or failures
- Cannot optimize model selection or detect cost anomalies

**Metrics to Add:**

```yaml
llm_provider_request_seconds:
  type: histogram
  unit: seconds
  labels:
    - provider: string (openai, anthropic, gemini)
    - model: string (gpt-4, claude-3, etc.)
    - outcome: enum[success, error, timeout, rate_limited]
  buckets: [0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120]
  description: "Provider API call latency"

llm_provider_tokens_total:
  type: counter
  labels:
    - provider: string
    - model: string
    - type: enum[prompt, completion]
  description: "Total tokens consumed by provider and type"

llm_provider_cost_usd_total:
  type: counter
  labels:
    - provider: string
    - model: string
  description: "Estimated cumulative cost in USD"

llm_provider_errors_total:
  type: counter
  labels:
    - provider: string
    - model: string
    - error_type: enum[auth, rate_limit, invalid_request, server_error, timeout]
  description: "Provider errors by category"

llm_provider_rate_limit_delays_seconds:
  type: histogram
  labels:
    - provider: string
  buckets: [0.1, 0.5, 1, 2, 5, 10, 30, 60]
  description: "Duration spent waiting due to rate limits"
```

**Implementation:**

```go
// engine/llm/metrics/metrics.go (NEW FILE)
package metrics

import (
    "context"
    "sync"
    "time"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    requestLatency     metric.Float64Histogram
    tokensCounter      metric.Int64Counter
    costCounter        metric.Float64Counter
    errorsCounter      metric.Int64Counter
    rateLimitDelays    metric.Float64Histogram
)

func initMetrics() {
    meter := otel.GetMeterProvider().Meter("compozy.llm")

    requestLatency, _ = meter.Float64Histogram(
        "llm_provider_request_seconds",
        metric.WithDescription("Provider API call latency"),
        metric.WithUnit("s"),
        metric.WithExplicitBucketBoundaries(0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120),
    )

    tokensCounter, _ = meter.Int64Counter(
        "llm_provider_tokens_total",
        metric.WithDescription("Total tokens consumed"),
    )

    costCounter, _ = meter.Float64Counter(
        "llm_provider_cost_usd_total",
        metric.WithDescription("Estimated cost in USD"),
    )

    errorsCounter, _ = meter.Int64Counter(
        "llm_provider_errors_total",
        metric.WithDescription("Provider errors by category"),
    )

    rateLimitDelays, _ = meter.Float64Histogram(
        "llm_provider_rate_limit_delays_seconds",
        metric.WithDescription("Rate limit wait time"),
        metric.WithUnit("s"),
        metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 2, 5, 10, 30, 60),
    )
}

func RecordRequest(ctx context.Context, provider, model string, duration time.Duration, outcome string) {
    once.Do(initMetrics)

    requestLatency.Record(ctx, duration.Seconds(),
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("model", model),
            attribute.String("outcome", outcome),
        ))
}

func RecordTokens(ctx context.Context, provider, model, tokenType string, count int) {
    once.Do(initMetrics)

    tokensCounter.Add(ctx, int64(count),
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("model", model),
            attribute.String("type", tokenType),
        ))
}

func RecordCost(ctx context.Context, provider, model string, costUSD float64) {
    once.Do(initMetrics)

    costCounter.Add(ctx, costUSD,
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("model", model),
        ))
}

func RecordError(ctx context.Context, provider, model, errorType string) {
    once.Do(initMetrics)

    errorsCounter.Add(ctx, 1,
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("model", model),
            attribute.String("error_type", errorType),
        ))
}

func RecordRateLimitDelay(ctx context.Context, provider string, delay time.Duration) {
    once.Do(initMetrics)

    rateLimitDelays.Record(ctx, delay.Seconds(),
        metric.WithAttributes(attribute.String("provider", provider)))
}
```

**Usage in Provider:**

```go
// engine/llm/provider/openai.go
func (p *OpenAIProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    start := time.Now()

    resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    req.Model,
        Messages: convertMessages(req.Messages),
    })

    duration := time.Since(start)

    if err != nil {
        outcome := categorizeError(err)
        metrics.RecordRequest(ctx, "openai", req.Model, duration, "error")
        metrics.RecordError(ctx, "openai", req.Model, outcome)
        return nil, err
    }

    // Record success metrics
    metrics.RecordRequest(ctx, "openai", req.Model, duration, "success")

    // Record token usage
    metrics.RecordTokens(ctx, "openai", req.Model, "prompt", resp.Usage.PromptTokens)
    metrics.RecordTokens(ctx, "openai", req.Model, "completion", resp.Usage.CompletionTokens)

    // Record cost
    cost := calculateCost(req.Model, resp.Usage)
    metrics.RecordCost(ctx, "openai", req.Model, cost)

    return &CompletionResponse{
        Content: resp.Choices[0].Message.Content,
        Usage: &UsageMetadata{
            PromptTokens:     resp.Usage.PromptTokens,
            CompletionTokens: resp.Usage.CompletionTokens,
            TotalTokens:      resp.Usage.TotalTokens,
        },
    }, nil
}

func categorizeError(err error) string {
    if errors.Is(err, context.DeadlineExceeded) {
        return "timeout"
    }

    // OpenAI-specific error types
    var apiErr *openai.APIError
    if errors.As(err, &apiErr) {
        switch apiErr.HTTPStatusCode {
        case 401, 403:
            return "auth"
        case 429:
            return "rate_limit"
        case 400:
            return "invalid_request"
        case 500, 502, 503, 504:
            return "server_error"
        }
    }

    return "unknown"
}
```

**Dashboard Queries:**

```promql
# Provider latency by model
histogram_quantile(0.95,
  rate(llm_provider_request_seconds_bucket[5m])
) by (provider, model)

# Token consumption rate
rate(llm_provider_tokens_total[5m]) by (provider, model, type)

# Cost per minute
rate(llm_provider_cost_usd_total[1m]) * 60

# Error rate by type
rate(llm_provider_errors_total[5m]) by (error_type)

# Rate limit frequency
rate(llm_provider_errors_total{error_type="rate_limit"}[5m])
```

**Effort:** M (3-4h)
