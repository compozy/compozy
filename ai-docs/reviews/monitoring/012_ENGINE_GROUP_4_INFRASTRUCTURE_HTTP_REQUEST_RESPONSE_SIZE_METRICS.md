---
title: "HTTP Request/Response Size Metrics"
group: "ENGINE_GROUP_4_INFRASTRUCTURE"
category: "monitoring"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_4_INFRASTRUCTURE_MONITORING.md"
issue_index: "1"
sequence: "12"
---

## HTTP Request/Response Size Metrics

**Priority:** 🔴 CRITICAL

**Location:** `engine/infra/server/middleware/` (NEW)

**Why Critical:**

- Cannot detect large payload attacks
- No visibility into bandwidth usage
- Cannot optimize compression
- Missing cost attribution for cloud egress

**Metrics to Add:**

```yaml
http_request_size_bytes:
  type: histogram
  unit: bytes
  labels:
    - method: enum[GET, POST, PUT, DELETE, PATCH]
    - path: string (without IDs, e.g., "/api/workflows/:id")
    - status_code: int
  buckets: [100, 1000, 10000, 100000, 1000000, 10000000, 100000000]
  description: "Size distribution of HTTP request bodies"

http_response_size_bytes:
  type: histogram
  unit: bytes
  labels:
    - method: enum[GET, POST, PUT, DELETE, PATCH]
    - path: string
    - status_code: int
  buckets: [100, 1000, 10000, 100000, 1000000, 10000000, 100000000]
  description: "Size distribution of HTTP response bodies"

http_request_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - method: enum[GET, POST, PUT, DELETE, PATCH]
    - path: string
    - status_code: int
  buckets: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
  description: "HTTP request latency distribution"

http_requests_in_flight:
  type: gauge
  labels:
    - method: enum[GET, POST, PUT, DELETE, PATCH]
  description: "Number of HTTP requests currently being processed"
```

**Implementation:**

```go
// engine/infra/server/middleware/metrics.go (NEW FILE)
package middleware

import (
    "context"
    "net/http"
    "strconv"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    requestSize       metric.Int64Histogram
    responseSize      metric.Int64Histogram
    requestDuration   metric.Float64Histogram
    requestsInFlight  metric.Int64UpDownCounter
)

func initMetrics() {
    meter := otel.GetMeterProvider().Meter("compozy.http")

    once.Do(func() {
        var err error

        requestSize, err = meter.Int64Histogram(
            "compozy_http_request_size_bytes",
            metric.WithDescription("Size distribution of HTTP request bodies"),
            metric.WithUnit("bytes"),
        )
        if err != nil {
            panic(err)
        }

        responseSize, err = meter.Int64Histogram(
            "compozy_http_response_size_bytes",
            metric.WithDescription("Size distribution of HTTP response bodies"),
            metric.WithUnit("bytes"),
        )
        if err != nil {
            panic(err)
        }

        requestDuration, err = meter.Float64Histogram(
            "compozy_http_request_duration_seconds",
            metric.WithDescription("HTTP request latency distribution"),
            metric.WithUnit("seconds"),
        )
        if err != nil {
            panic(err)
        }

        requestsInFlight, err = meter.Int64UpDownCounter(
            "compozy_http_requests_in_flight",
            metric.WithDescription("Number of HTTP requests currently being processed"),
        )
        if err != nil {
            panic(err)
        }
    })
}

// MetricsMiddleware records HTTP request metrics
func MetricsMiddleware() gin.HandlerFunc {
    initMetrics()

    return func(c *gin.Context) {
        start := time.Now()

        // Normalize path (remove IDs)
        path := normalizePath(c.FullPath())
        method := c.Request.Method

        // Track in-flight requests
        attrs := []attribute.KeyValue{
            attribute.String("method", method),
        }
        requestsInFlight.Add(c.Request.Context(), 1, metric.WithAttributes(attrs...))
        defer requestsInFlight.Add(c.Request.Context(), -1, metric.WithAttributes(attrs...))

        // Record request size
        if c.Request.ContentLength > 0 {
            requestSize.Record(c.Request.Context(), c.Request.ContentLength,
                metric.WithAttributes(
                    attribute.String("method", method),
                    attribute.String("path", path),
                ))
        }

        // Process request
        c.Next()

        // Record metrics after request completes
        duration := time.Since(start).Seconds()
        statusCode := c.Writer.Status()

        fullAttrs := []attribute.KeyValue{
            attribute.String("method", method),
            attribute.String("path", path),
            attribute.Int("status_code", statusCode),
        }

        requestDuration.Record(c.Request.Context(), duration,
            metric.WithAttributes(fullAttrs...))

        // Record response size
        responseSize.Record(c.Request.Context(), int64(c.Writer.Size()),
            metric.WithAttributes(fullAttrs...))
    }
}

// normalizePath removes IDs and parameters from path for cardinality control
func normalizePath(path string) string {
    if path == "" {
        return "/"
    }
    // Gin already provides normalized paths with route parameters
    // e.g., "/api/workflows/:id" instead of "/api/workflows/123"
    return path
}
```

**Register middleware:**

```go
// engine/infra/server/router.go
import "github.com/compozy/compozy/engine/infra/server/middleware"

func (s *Server) setupMiddleware() {
    // ... existing middleware ...

    // Metrics middleware should be early in chain
    s.router.Use(middleware.MetricsMiddleware())

    // ... other middleware ...
}
```

**PromQL Queries:**

```promql
# Average request size by endpoint
rate(compozy_http_request_size_bytes_sum[5m])
  / rate(compozy_http_request_size_bytes_count[5m])

# P95 response size
histogram_quantile(0.95,
  rate(compozy_http_response_size_bytes_bucket[5m]))

# Large responses (>10MB)
sum(rate(compozy_http_response_size_bytes_bucket{le="10000000"}[5m]))
  - sum(rate(compozy_http_response_size_bytes_bucket{le="1000000"}[5m]))

# Slowest endpoints (P99 latency)
topk(10,
  histogram_quantile(0.99,
    rate(compozy_http_request_duration_seconds_bucket[5m])))

# In-flight requests spike detection
max_over_time(compozy_http_requests_in_flight[5m]) > 100
```

**Alerting:**

```yaml
# prometheus/alerts.yml
groups:
  - name: http_performance
    rules:
      - alert: HighRequestLatency
        expr: |
          histogram_quantile(0.95, 
            rate(compozy_http_request_duration_seconds_bucket[5m])) > 2
        for: 5m
        annotations:
          summary: "P95 request latency above 2s"

      - alert: LargeResponsePayloads
        expr: |
          rate(compozy_http_response_size_bytes_sum[5m]) 
            / rate(compozy_http_response_size_bytes_count[5m]) > 10000000
        for: 10m
        annotations:
          summary: "Average response size above 10MB"
```

**Effort:** M (3h)  
**Risk:** Low - middleware addition
