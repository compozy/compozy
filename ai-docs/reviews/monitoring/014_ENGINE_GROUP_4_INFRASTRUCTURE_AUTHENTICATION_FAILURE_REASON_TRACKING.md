---
title: "Authentication Failure Reason Tracking"
group: "ENGINE_GROUP_4_INFRASTRUCTURE"
category: "monitoring"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_4_INFRASTRUCTURE_MONITORING.md"
issue_index: "3"
sequence: "14"
---

## Authentication Failure Reason Tracking

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
