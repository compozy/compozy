---
title: "Webhook Payload Size and Processing Metrics"
group: "ENGINE_GROUP_4_INFRASTRUCTURE"
category: "monitoring"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_4_INFRASTRUCTURE_MONITORING.md"
issue_index: "4"
sequence: "15"
---

## Webhook Payload Size and Processing Metrics

**Priority:** 🟡 MEDIUM

**Location:** `engine/webhook/metrics.go` (NEW FILE)

**Metrics to Add:**

```yaml
webhook_payload_size_bytes:
  type: histogram
  unit: bytes
  labels:
    - event_type: string
    - source: string
  buckets: [100, 1000, 10000, 100000, 1000000]
  description: "Size distribution of webhook payloads"

webhook_processing_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - event_type: string
    - outcome: enum[success, error]
  buckets: [0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5]
  description: "Time to process webhook events"

webhook_events_total:
  type: counter
  labels:
    - event_type: string
    - outcome: enum[success, error]
  description: "Total webhook events received"

webhook_queue_depth:
  type: gauge
  description: "Number of webhook events waiting to be processed"
```

**Implementation:**

```go
// engine/webhook/metrics.go (NEW FILE)
package webhook

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

    payloadSize         metric.Int64Histogram
    processingDuration  metric.Float64Histogram
    eventsTotal         metric.Int64Counter
    queueDepth          metric.Int64UpDownCounter
)

func initMetrics() {
    meter := otel.GetMeterProvider().Meter("compozy.webhook")

    once.Do(func() {
        var err error

        payloadSize, err = meter.Int64Histogram(
            "compozy_webhook_payload_size_bytes",
            metric.WithDescription("Size distribution of webhook payloads"),
            metric.WithUnit("bytes"),
        )
        if err != nil {
            panic(err)
        }

        processingDuration, err = meter.Float64Histogram(
            "compozy_webhook_processing_duration_seconds",
            metric.WithDescription("Time to process webhook events"),
            metric.WithUnit("seconds"),
        )
        if err != nil {
            panic(err)
        }

        eventsTotal, err = meter.Int64Counter(
            "compozy_webhook_events_total",
            metric.WithDescription("Total webhook events received"),
        )
        if err != nil {
            panic(err)
        }

        queueDepth, err = meter.Int64UpDownCounter(
            "compozy_webhook_queue_depth",
            metric.WithDescription("Number of webhook events waiting to be processed"),
        )
        if err != nil {
            panic(err)
        }
    })
}

// RecordWebhookEvent records metrics for a webhook event
func RecordWebhookEvent(ctx context.Context, eventType string, source string, payloadBytes int, duration time.Duration, err error) {
    initMetrics()

    outcome := "success"
    if err != nil {
        outcome = "error"
    }

    attrs := []attribute.KeyValue{
        attribute.String("event_type", eventType),
    }

    // Record payload size
    payloadSize.Record(ctx, int64(payloadBytes),
        metric.WithAttributes(
            attribute.String("event_type", eventType),
            attribute.String("source", source),
        ))

    // Record processing duration
    processingDuration.Record(ctx, duration.Seconds(),
        metric.WithAttributes(
            attribute.String("event_type", eventType),
            attribute.String("outcome", outcome),
        ))

    // Increment events counter
    eventsTotal.Add(ctx, 1,
        metric.WithAttributes(
            attribute.String("event_type", eventType),
            attribute.String("outcome", outcome),
        ))
}

// IncrementQueueDepth increments the webhook queue depth
func IncrementQueueDepth(ctx context.Context) {
    initMetrics()
    queueDepth.Add(ctx, 1)
}

// DecrementQueueDepth decrements the webhook queue depth
func DecrementQueueDepth(ctx context.Context) {
    initMetrics()
    queueDepth.Add(ctx, -1)
}
```

**PromQL Queries:**

```promql
# Average webhook payload size
rate(compozy_webhook_payload_size_bytes_sum[5m])
  / rate(compozy_webhook_payload_size_bytes_count[5m])

# Webhook processing latency P95
histogram_quantile(0.95,
  rate(compozy_webhook_processing_duration_seconds_bucket[5m]))

# Webhook success rate
rate(compozy_webhook_events_total{outcome="success"}[5m])
  / rate(compozy_webhook_events_total[5m]) * 100

# Queue backlog
max_over_time(compozy_webhook_queue_depth[5m])
```

**Effort:** S (2h)  
**Risk:** None
