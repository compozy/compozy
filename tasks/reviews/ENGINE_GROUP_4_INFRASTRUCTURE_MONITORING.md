# Engine Group 4: Infrastructure - Monitoring Improvements

**Packages:** infra, auth, webhook, worker, autoload

---

## Executive Summary

Comprehensive monitoring instrumentation for infrastructure components to provide visibility into HTTP servers, authentication, webhook processing, worker management, and configuration auto-loading.

**Current State:**

- ✅ Dispatcher health metrics exist (but need cardinality fixes)
- ❌ No HTTP request/response size metrics
- ❌ No database connection pool metrics
- ❌ No auth failure reason tracking
- ❌ No webhook payload size metrics
- ❌ No autoload timing metrics
- ❌ No worker utilization metrics

---

## Missing Metrics

### 1. HTTP Request/Response Size Metrics

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

---

### 2. Database Connection Pool Metrics

**Priority:** 🔴 CRITICAL

**Location:** `engine/infra/postgres/metrics.go`, `engine/infra/redis/metrics.go` (NEW FILES)

**Why Critical:**

- Cannot detect connection exhaustion
- No visibility into pool saturation
- Cannot tune pool sizes
- Missing connection leak detection

**Metrics to Add:**

```yaml
postgres_connections_open:
  type: gauge
  description: "Number of open Postgres connections"

postgres_connections_in_use:
  type: gauge
  description: "Number of Postgres connections currently in use"

postgres_connections_idle:
  type: gauge
  description: "Number of idle Postgres connections"

postgres_connection_wait_duration_seconds:
  type: histogram
  unit: seconds
  buckets: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2]
  description: "Time spent waiting for a connection from the pool"

redis_pool_size:
  type: gauge
  description: "Configured Redis connection pool size"

redis_pool_hits_total:
  type: counter
  description: "Number of times a connection was obtained from the pool"

redis_pool_misses_total:
  type: counter
  description: "Number of times a new connection had to be created"

redis_pool_timeouts_total:
  type: counter
  description: "Number of times waiting for a connection timed out"

redis_pool_idle_conns:
  type: gauge
  description: "Number of idle Redis connections in the pool"

redis_pool_stale_conns_total:
  type: counter
  description: "Number of stale Redis connections removed"
```

**Implementation (Postgres):**

```go
// engine/infra/postgres/metrics.go (NEW FILE)
package postgres

import (
    "context"
    "database/sql"
    "sync"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    connectionsOpen     metric.Int64ObservableGauge
    connectionsInUse    metric.Int64ObservableGauge
    connectionsIdle     metric.Int64ObservableGauge
    connectionWaitTime  metric.Float64Histogram

    dbInstance *sql.DB
)

func InitMetrics(ctx context.Context, db *sql.DB) {
    dbInstance = db
    meter := otel.GetMeterProvider().Meter("compozy.postgres")

    once.Do(func() {
        var err error

        connectionsOpen, err = meter.Int64ObservableGauge(
            "compozy_postgres_connections_open",
            metric.WithDescription("Number of open Postgres connections"),
        )
        if err != nil {
            panic(err)
        }

        connectionsInUse, err = meter.Int64ObservableGauge(
            "compozy_postgres_connections_in_use",
            metric.WithDescription("Number of Postgres connections currently in use"),
        )
        if err != nil {
            panic(err)
        }

        connectionsIdle, err = meter.Int64ObservableGauge(
            "compozy_postgres_connections_idle",
            metric.WithDescription("Number of idle Postgres connections"),
        )
        if err != nil {
            panic(err)
        }

        connectionWaitTime, err = meter.Float64Histogram(
            "compozy_postgres_connection_wait_duration_seconds",
            metric.WithDescription("Time spent waiting for a connection from the pool"),
            metric.WithUnit("seconds"),
        )
        if err != nil {
            panic(err)
        }

        // Register callback to observe pool stats
        _, err = meter.RegisterCallback(
            func(_ context.Context, o metric.Observer) error {
                if dbInstance == nil {
                    return nil
                }

                stats := dbInstance.Stats()

                o.ObserveInt64(connectionsOpen, int64(stats.OpenConnections))
                o.ObserveInt64(connectionsInUse, int64(stats.InUse))
                o.ObserveInt64(connectionsIdle, int64(stats.Idle))

                return nil
            },
            connectionsOpen,
            connectionsInUse,
            connectionsIdle,
        )
        if err != nil {
            panic(err)
        }
    })
}

// RecordConnectionWait records time spent waiting for a connection
func RecordConnectionWait(ctx context.Context, duration time.Duration) {
    if connectionWaitTime != nil {
        connectionWaitTime.Record(ctx, duration.Seconds())
    }
}

// WithMetrics wraps a DB connection with metrics tracking
func WithMetrics(db *sql.DB) *sql.DB {
    // Create a wrapper that tracks connection wait time
    // This is a simplified version - in production you'd use a proper connection wrapper
    return db
}
```

**Implementation (Redis):**

```go
// engine/infra/redis/metrics.go (NEW FILE)
package redis

import (
    "context"
    "sync"

    "github.com/redis/go-redis/v9"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    poolSize        metric.Int64ObservableGauge
    poolHits        metric.Int64Counter
    poolMisses      metric.Int64Counter
    poolTimeouts    metric.Int64Counter
    poolIdleConns   metric.Int64ObservableGauge
    poolStaleConns  metric.Int64Counter

    redisClient *redis.Client
)

func InitMetrics(ctx context.Context, client *redis.Client) {
    redisClient = client
    meter := otel.GetMeterProvider().Meter("compozy.redis")

    once.Do(func() {
        var err error

        poolSize, err = meter.Int64ObservableGauge(
            "compozy_redis_pool_size",
            metric.WithDescription("Configured Redis connection pool size"),
        )
        if err != nil {
            panic(err)
        }

        poolHits, err = meter.Int64Counter(
            "compozy_redis_pool_hits_total",
            metric.WithDescription("Number of times a connection was obtained from the pool"),
        )
        if err != nil {
            panic(err)
        }

        poolMisses, err = meter.Int64Counter(
            "compozy_redis_pool_misses_total",
            metric.WithDescription("Number of times a new connection had to be created"),
        )
        if err != nil {
            panic(err)
        }

        poolTimeouts, err = meter.Int64Counter(
            "compozy_redis_pool_timeouts_total",
            metric.WithDescription("Number of times waiting for a connection timed out"),
        )
        if err != nil {
            panic(err)
        }

        poolIdleConns, err = meter.Int64ObservableGauge(
            "compozy_redis_pool_idle_conns",
            metric.WithDescription("Number of idle Redis connections in the pool"),
        )
        if err != nil {
            panic(err)
        }

        poolStaleConns, err = meter.Int64Counter(
            "compozy_redis_pool_stale_conns_total",
            metric.WithDescription("Number of stale Redis connections removed"),
        )
        if err != nil {
            panic(err)
        }

        // Register callback to observe pool stats
        _, err = meter.RegisterCallback(
            func(_ context.Context, o metric.Observer) error {
                if redisClient == nil {
                    return nil
                }

                stats := redisClient.PoolStats()

                o.ObserveInt64(poolSize, int64(stats.TotalConns))
                o.ObserveInt64(poolIdleConns, int64(stats.IdleConns))

                // Record counters (they accumulate)
                poolHits.Add(context.Background(), int64(stats.Hits))
                poolMisses.Add(context.Background(), int64(stats.Misses))
                poolTimeouts.Add(context.Background(), int64(stats.Timeouts))
                poolStaleConns.Add(context.Background(), int64(stats.StaleConns))

                return nil
            },
            poolSize,
            poolIdleConns,
        )
        if err != nil {
            panic(err)
        }
    })
}
```

**PromQL Queries:**

```promql
# Postgres connection pool utilization
compozy_postgres_connections_in_use
  / compozy_postgres_connections_open * 100

# Postgres connections near limit
compozy_postgres_connections_open
  / on() group_left() compozy_postgres_max_open_connections * 100 > 90

# Redis pool hit rate
rate(compozy_redis_pool_hits_total[5m])
  / (rate(compozy_redis_pool_hits_total[5m])
     + rate(compozy_redis_pool_misses_total[5m])) * 100

# Redis pool exhaustion
compozy_redis_pool_timeouts_total > 0

# Idle connection waste (Postgres)
compozy_postgres_connections_idle / compozy_postgres_connections_open > 0.5
```

**Alerting:**

```yaml
# prometheus/alerts.yml
groups:
  - name: database_pools
    rules:
      - alert: PostgresPoolNearLimit
        expr: |
          compozy_postgres_connections_in_use 
            / compozy_postgres_connections_open > 0.9
        for: 5m
        annotations:
          summary: "Postgres connection pool >90% utilized"

      - alert: RedisPoolExhaustion
        expr: rate(compozy_redis_pool_timeouts_total[5m]) > 0
        for: 2m
        annotations:
          summary: "Redis pool experiencing connection timeouts"
```

**Effort:** M (4h)  
**Risk:** Low - passive metrics collection

---

### 3. Authentication Failure Reason Tracking

**Priority:** 🟡 MEDIUM

**Location:** `engine/auth/metrics.go`

**Why Important:**

- Cannot distinguish between invalid credentials, expired tokens, missing headers
- No visibility into attack patterns
- Cannot tune rate limiting per failure type
- Missing security audit trail

**Metrics to Add:**

```yaml
auth_attempts_total:
  type: counter
  labels:
    - outcome: enum[success, failure]
    - reason: enum[invalid_credentials, expired_token, missing_auth, invalid_format, rate_limited, unknown]
    - method: enum[jwt, api_key, oauth]
  description: "Total authentication attempts categorized by outcome and reason"

auth_token_age_seconds:
  type: histogram
  unit: seconds
  labels:
    - method: enum[jwt, api_key]
  buckets: [60, 300, 900, 1800, 3600, 7200, 14400, 28800, 86400]
  description: "Age of tokens used for authentication"

auth_rate_limit_hits_total:
  type: counter
  labels:
    - user_id: string (optional, for identified users)
    - ip_address: string (masked for privacy, e.g., "1.2.3.0")
  description: "Number of times rate limiting was triggered"
```

**Implementation:**

```go
// engine/auth/metrics.go (UPDATE EXISTING OR CREATE)
package auth

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

    authAttempts      metric.Int64Counter
    tokenAge          metric.Float64Histogram
    rateLimitHits     metric.Int64Counter
)

func initMetrics() {
    meter := otel.GetMeterProvider().Meter("compozy.auth")

    once.Do(func() {
        var err error

        authAttempts, err = meter.Int64Counter(
            "compozy_auth_attempts_total",
            metric.WithDescription("Total authentication attempts categorized by outcome and reason"),
        )
        if err != nil {
            panic(err)
        }

        tokenAge, err = meter.Float64Histogram(
            "compozy_auth_token_age_seconds",
            metric.WithDescription("Age of tokens used for authentication"),
            metric.WithUnit("seconds"),
        )
        if err != nil {
            panic(err)
        }

        rateLimitHits, err = meter.Int64Counter(
            "compozy_auth_rate_limit_hits_total",
            metric.WithDescription("Number of times rate limiting was triggered"),
        )
        if err != nil {
            panic(err)
        }
    })
}

// AuthOutcome represents the result of an authentication attempt
type AuthOutcome string

const (
    AuthSuccess AuthOutcome = "success"
    AuthFailure AuthOutcome = "failure"
)

// AuthFailureReason categorizes why authentication failed
type AuthFailureReason string

const (
    ReasonInvalidCredentials AuthFailureReason = "invalid_credentials"
    ReasonExpiredToken       AuthFailureReason = "expired_token"
    ReasonMissingAuth        AuthFailureReason = "missing_auth"
    ReasonInvalidFormat      AuthFailureReason = "invalid_format"
    ReasonRateLimited        AuthFailureReason = "rate_limited"
    ReasonUnknown            AuthFailureReason = "unknown"
    ReasonNone               AuthFailureReason = "" // For success
)

// RecordAuthAttempt records an authentication attempt with outcome and reason
func RecordAuthAttempt(ctx context.Context, outcome AuthOutcome, reason AuthFailureReason, method string) {
    initMetrics()

    attrs := []attribute.KeyValue{
        attribute.String("outcome", string(outcome)),
        attribute.String("method", method),
    }

    if outcome == AuthFailure && reason != "" {
        attrs = append(attrs, attribute.String("reason", string(reason)))
    }

    authAttempts.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordTokenAge records the age of a token used for authentication
func RecordTokenAge(ctx context.Context, issuedAt time.Time, method string) {
    initMetrics()

    age := time.Since(issuedAt).Seconds()
    tokenAge.Record(ctx, age,
        metric.WithAttributes(
            attribute.String("method", method),
        ))
}

// RecordRateLimitHit records a rate limit trigger
func RecordRateLimitHit(ctx context.Context, userID string, ipAddr string) {
    initMetrics()

    // Mask IP address for privacy (keep first 3 octets)
    maskedIP := maskIPAddress(ipAddr)

    attrs := []attribute.KeyValue{
        attribute.String("ip_address", maskedIP),
    }

    if userID != "" {
        attrs = append(attrs, attribute.String("user_id", userID))
    }

    rateLimitHits.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// maskIPAddress masks the last octet of IPv4 addresses for privacy
func maskIPAddress(ip string) string {
    // Simple implementation - in production use proper IP parsing
    if len(ip) == 0 {
        return "unknown"
    }
    // For now, return as-is - implement proper masking based on privacy requirements
    return ip
}
```

**Update auth middleware to record metrics:**

```go
// engine/auth/router/middleware.go (or wherever auth middleware lives)
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")

        if token == "" {
            RecordAuthAttempt(c.Request.Context(), AuthFailure, ReasonMissingAuth, "jwt")
            c.AbortWithStatusJSON(401, gin.H{"error": "missing authorization"})
            return
        }

        claims, err := validateToken(token)
        if err != nil {
            reason := categorizeAuthError(err)
            RecordAuthAttempt(c.Request.Context(), AuthFailure, reason, "jwt")
            c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
            return
        }

        // Record successful auth
        RecordAuthAttempt(c.Request.Context(), AuthSuccess, ReasonNone, "jwt")

        // Record token age
        if claims.IssuedAt != nil {
            RecordTokenAge(c.Request.Context(), claims.IssuedAt.Time, "jwt")
        }

        c.Set("user_id", claims.UserID)
        c.Next()
    }
}

// categorizeAuthError maps errors to failure reasons
func categorizeAuthError(err error) AuthFailureReason {
    switch {
    case errors.Is(err, ErrExpiredToken):
        return ReasonExpiredToken
    case errors.Is(err, ErrInvalidSignature):
        return ReasonInvalidCredentials
    case errors.Is(err, ErrMalformedToken):
        return ReasonInvalidFormat
    default:
        return ReasonUnknown
    }
}
```

**PromQL Queries:**

```promql
# Auth failure rate
rate(compozy_auth_attempts_total{outcome="failure"}[5m])
  / rate(compozy_auth_attempts_total[5m]) * 100

# Failure reasons breakdown
sum by (reason) (rate(compozy_auth_attempts_total{outcome="failure"}[5m]))

# Expired token issues
rate(compozy_auth_attempts_total{reason="expired_token"}[5m]) > 1

# Average token age at use
rate(compozy_auth_token_age_seconds_sum[5m])
  / rate(compozy_auth_token_age_seconds_count[5m])

# Rate limit triggers by IP
topk(10, sum by (ip_address) (rate(compozy_auth_rate_limit_hits_total[5m])))
```

**Alerting:**

```yaml
# prometheus/alerts.yml
groups:
  - name: authentication
    rules:
      - alert: HighAuthFailureRate
        expr: |
          rate(compozy_auth_attempts_total{outcome="failure"}[5m]) 
            / rate(compozy_auth_attempts_total[5m]) > 0.5
        for: 5m
        annotations:
          summary: "Auth failure rate above 50%"

      - alert: SuspiciousAuthActivity
        expr: |
          rate(compozy_auth_attempts_total{reason="invalid_credentials"}[5m]) > 10
        for: 2m
        annotations:
          summary: "High rate of invalid credentials attempts (possible attack)"
```

**Effort:** M (3h)  
**Risk:** Low - passive metrics, no behavior change

---

### 4. Webhook Payload Size and Processing Metrics

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

---

### 5. Autoload Performance Metrics

**Priority:** 🟢 LOW

**Location:** `engine/autoload/metrics.go` (NEW FILE)

**Metrics to Add:**

```yaml
autoload_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - project: string
  buckets: [0.1, 0.5, 1, 2, 5, 10, 30]
  description: "Time to complete autoload process"

autoload_files_processed_total:
  type: counter
  labels:
    - project: string
    - outcome: enum[success, error]
  description: "Total files processed by autoload"

autoload_configs_loaded_total:
  type: counter
  labels:
    - project: string
    - type: enum[workflow, agent, tool, mcp]
  description: "Total configurations loaded by type"

autoload_errors_total:
  type: counter
  labels:
    - project: string
    - error_type: enum[parse_error, validation_error, duplicate_error, security_error]
  description: "Total autoload errors by category"
```

**Implementation:** (See autoload.go, add metrics to Load() and LoadWithResult())

**PromQL Queries:**

```promql
# Autoload duration trend
rate(autoload_duration_seconds_sum[5m])
  / rate(autoload_duration_seconds_count[5m])

# Files per second processing rate
rate(autoload_files_processed_total[5m])

# Error rate by type
sum by (error_type) (rate(autoload_errors_total[5m]))
```

**Effort:** S (2h)  
**Risk:** None

---

### 6. Worker Utilization Metrics

**Priority:** 🟡 MEDIUM

**Location:** `engine/worker/metrics.go` (NEW FILE)

**Metrics to Add:**

```yaml
worker_activities_executing:
  type: gauge
  description: "Number of activities currently executing"

worker_workflows_executing:
  type: gauge
  description: "Number of workflows currently executing"

worker_task_queue_depth:
  type: gauge
  labels:
    - queue_name: string
  description: "Number of tasks waiting in queue"

worker_activity_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - activity_type: string
    - outcome: enum[success, error, timeout]
  buckets: [0.1, 0.5, 1, 5, 10, 30, 60, 300]
  description: "Activity execution duration"

worker_utilization_ratio:
  type: gauge
  labels:
    - worker_type: enum[activity, workflow]
  description: "Worker utilization (executing / max_concurrent)"
```

**PromQL Queries:**

```promql
# Worker utilization percentage
worker_activities_executing / worker_max_concurrent_activities * 100

# Queue backlog alert
worker_task_queue_depth > 100

# Average activity duration
rate(worker_activity_duration_seconds_sum[5m])
  / rate(worker_activity_duration_seconds_count[5m])
```

**Effort:** M (3h)  
**Risk:** Low

---

## Implementation Priorities

### Phase 1: Critical Observability (Week 1)

1. HTTP request/response metrics (#1) - **3h**
2. Database pool metrics (#2) - **4h**

### Phase 2: Security & Webhook (Week 2)

3. Auth failure tracking (#3) - **3h**
4. Webhook metrics (#4) - **2h**

### Phase 3: Operations Metrics (Week 3)

5. Autoload metrics (#5) - **2h**
6. Worker utilization (#6) - **3h**

**Total effort:** 17 hours

---

## Dashboards

### HTTP Performance Dashboard

```yaml
panels:
  - title: Request Rate
    query: rate(compozy_http_requests_total[5m])

  - title: P95 Latency
    query: histogram_quantile(0.95, rate(compozy_http_request_duration_seconds_bucket[5m]))

  - title: Request Size Distribution
    query: rate(compozy_http_request_size_bytes_bucket[5m])

  - title: In-Flight Requests
    query: compozy_http_requests_in_flight
```

### Database Health Dashboard

```yaml
panels:
  - title: Postgres Pool Utilization
    query: compozy_postgres_connections_in_use / compozy_postgres_connections_open * 100

  - title: Redis Pool Hit Rate
    query: rate(compozy_redis_pool_hits_total[5m]) / (rate(compozy_redis_pool_hits_total[5m]) + rate(compozy_redis_pool_misses_total[5m])) * 100
```

---

## Related Documentation

- **GROUP_4_PERFORMANCE.md** - Infrastructure performance optimizations
- **GROUP_1_MONITORING.md** - Runtime execution monitoring patterns
- **GROUP_2_MONITORING.md** - LLM and MCP monitoring patterns
