---
title: "Embedding Generation Metrics"
group: "ENGINE_GROUP_3_KNOWLEDGE_MEMORY"
category: "monitoring"
priority: "🔴 HIGH"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_3_KNOWLEDGE_MEMORY_MONITORING.md"
issue_index: "2"
sequence: "9"
---

## Embedding Generation Metrics

**Priority:** 🔴 HIGH

**Location:** `engine/memory/embeddings/`

**Metrics to Add:**

```yaml
embeddings_generate_seconds:
  type: histogram
  unit: seconds
  labels:
    - provider: string (openai, cohere)
    - model: string
    - batch_size: int
  buckets: [0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5]
  description: "Embedding generation latency"

embeddings_tokens_total:
  type: counter
  labels:
    - provider: string
    - model: string
  description: "Total tokens processed for embeddings"

embeddings_cache_hits_total:
  type: counter
  labels:
    - provider: string
  description: "Embedding cache hits"

embeddings_cache_misses_total:
  type: counter
  labels:
    - provider: string
  description: "Embedding cache misses"

embeddings_errors_total:
  type: counter
  labels:
    - provider: string
    - error_type: enum[auth, rate_limit, invalid_input, server_error]
  description: "Embedding generation errors"
```

**Implementation:**

```go
// engine/memory/embeddings/metrics.go (NEW)
package embeddings

import (
    "context"
    "sync"
    "time"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    generateLatency metric.Float64Histogram
    tokensCounter   metric.Int64Counter
    cacheHits       metric.Int64Counter
    cacheMisses     metric.Int64Counter
    errorsCounter   metric.Int64Counter
)

func initMetrics() {
    meter := otel.GetMeterProvider().Meter("compozy.embeddings")

    generateLatency, _ = meter.Float64Histogram(
        "embeddings_generate_seconds",
        metric.WithDescription("Embedding generation latency"),
        metric.WithUnit("s"),
    )

    tokensCounter, _ = meter.Int64Counter(
        "embeddings_tokens_total",
        metric.WithDescription("Tokens processed"),
    )

    cacheHits, _ = meter.Int64Counter(
        "embeddings_cache_hits_total",
        metric.WithDescription("Cache hits"),
    )

    cacheMisses, _ = meter.Int64Counter(
        "embeddings_cache_misses_total",
        metric.WithDescription("Cache misses"),
    )

    errorsCounter, _ = meter.Int64Counter(
        "embeddings_errors_total",
        metric.WithDescription("Generation errors"),
    )
}

func RecordGeneration(ctx context.Context, provider, model string, batchSize int, duration time.Duration, tokenCount int) {
    once.Do(initMetrics)

    generateLatency.Record(ctx, duration.Seconds(),
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("model", model),
            attribute.Int("batch_size", batchSize),
        ))

    tokensCounter.Add(ctx, int64(tokenCount),
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("model", model),
        ))
}

func RecordCacheHit(ctx context.Context, provider string) {
    once.Do(initMetrics)
    cacheHits.Add(ctx, 1, metric.WithAttributes(attribute.String("provider", provider)))
}

func RecordCacheMiss(ctx context.Context, provider string) {
    once.Do(initMetrics)
    cacheMisses.Add(ctx, 1, metric.WithAttributes(attribute.String("provider", provider)))
}

func RecordError(ctx context.Context, provider, errorType string) {
    once.Do(initMetrics)

    errorsCounter.Add(ctx, 1,
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("error_type", errorType),
        ))
}
```

**Usage:**

```go
// In cached_embedder.go
func (e *CachedEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    key := hashText(text)

    if embedding, ok := e.cache.Get(key); ok {
        metrics.RecordCacheHit(ctx, e.provider)
        return embedding, nil
    }

    metrics.RecordCacheMiss(ctx, e.provider)

    start := time.Now()
    embedding, tokenCount, err := e.underlying.Embed(ctx, text)
    if err != nil {
        metrics.RecordError(ctx, e.provider, categorizeError(err))
        return nil, err
    }

    metrics.RecordGeneration(ctx, e.provider, e.model, 1, time.Since(start), tokenCount)

    e.cache.Add(key, embedding)
    return embedding, nil
}
```

**Dashboard Queries:**

```promql
# Embedding latency by batch size
histogram_quantile(0.95,
  rate(embeddings_generate_seconds_bucket[5m])
) by (batch_size)

# Cache hit rate
rate(embeddings_cache_hits_total[5m]) /
(rate(embeddings_cache_hits_total[5m]) + rate(embeddings_cache_misses_total[5m]))

# Token usage rate
rate(embeddings_tokens_total[5m]) by (provider, model)
```

**Effort:** M (2-3h)
