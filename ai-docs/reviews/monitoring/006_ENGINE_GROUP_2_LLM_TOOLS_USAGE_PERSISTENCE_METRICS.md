---
title: "Usage Persistence Metrics"
group: "ENGINE_GROUP_2_LLM_TOOLS"
category: "monitoring"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_2_LLM_TOOLS_MONITORING.md"
issue_index: "3"
sequence: "6"
---

## Usage Persistence Metrics

**Priority:** 🟡 MEDIUM

**Location:** `engine/llm/usage/repository.go`

**Metrics to Add:**

```yaml
llm_usage_persist_seconds:
  type: histogram
  unit: seconds
  labels:
    - outcome: enum[success, error]
  buckets: [0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1]
  description: "Usage record persistence latency"

llm_usage_persist_errors_total:
  type: counter
  labels:
    - error_type: enum[db_error, timeout, validation]
  description: "Usage persistence failures"

llm_usage_persist_queue_size:
  type: gauge
  description: "Pending usage records waiting to be persisted"
```

**Implementation:**

```go
// engine/llm/usage/metrics.go (NEW)
package usage

import (
    "context"
    "sync"
    "time"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    persistLatency metric.Float64Histogram
    persistErrors  metric.Int64Counter
    queueSize      metric.Int64UpDownCounter
)

func initMetrics() {
    meter := otel.GetMeterProvider().Meter("compozy.llm.usage")

    persistLatency, _ = meter.Float64Histogram(
        "llm_usage_persist_seconds",
        metric.WithDescription("Usage persistence latency"),
        metric.WithUnit("s"),
    )

    persistErrors, _ = meter.Int64Counter(
        "llm_usage_persist_errors_total",
        metric.WithDescription("Persistence failures"),
    )

    queueSize, _ = meter.Int64UpDownCounter(
        "llm_usage_persist_queue_size",
        metric.WithDescription("Pending usage records"),
    )
}

func RecordPersist(ctx context.Context, duration time.Duration, err error) {
    once.Do(initMetrics)

    outcome := "success"
    if err != nil {
        outcome = "error"
    }

    persistLatency.Record(ctx, duration.Seconds(),
        metric.WithAttributes(attribute.String("outcome", outcome)))

    if err != nil {
        errorType := categorizeError(err)
        persistErrors.Add(ctx, 1,
            metric.WithAttributes(attribute.String("error_type", errorType)))
    }
}

func IncrementQueueSize(ctx context.Context) {
    once.Do(initMetrics)
    queueSize.Add(ctx, 1)
}

func DecrementQueueSize(ctx context.Context) {
    once.Do(initMetrics)
    queueSize.Add(ctx, -1)
}
```

**Dashboard Queries:**

```promql
# Persistence latency p99
histogram_quantile(0.99,
  rate(llm_usage_persist_seconds_bucket[5m])
)

# Error rate
rate(llm_usage_persist_errors_total[5m]) by (error_type)

# Queue backlog
llm_usage_persist_queue_size
```

**Effort:** S (1-2h)
