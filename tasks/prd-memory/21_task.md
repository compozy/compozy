---
status: pending
---

<task_context>
<domain>engine/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>resilience_libraries</dependencies>
</task_context>

# Task 21.0: Implement Enhanced Circuit Breaker and Resilience

## Overview

Replace the custom circuit breaker implementation in the privacy manager with `github.com/slok/goresilience` to provide production-grade resilience patterns including retry, timeout, and circuit breaker functionality with built-in Prometheus metrics.

## Subtasks

- [ ] 21.1 Add goresilience library dependency to go.mod
- [ ] 21.2 Replace custom circuit breaker in privacy manager
- [ ] 21.3 Add retry and timeout middleware for resilience
- [ ] 21.4 Integrate Prometheus metrics for monitoring
- [ ] 21.5 Add comprehensive tests for resilience patterns
- [ ] 21.6 Add configuration for resilience parameters
- [ ] 21.7 **NEW**: Conduct performance testing under various load conditions
- [ ] 21.8 **NEW**: Tune circuit breaker thresholds and timeout values
- [ ] 21.9 **NEW**: Add load testing scenarios for optimal configuration

## Implementation Details

### Library Dependencies

Add to `go.mod`:

```go
require (
    github.com/slok/goresilience v0.2.0    // Resilience patterns with metrics
)
```

### Enhanced Privacy Manager with Resilience

```go
// engine/memory/privacy/resilient_manager.go
package privacy

import (
    "context"
    "time"

    "github.com/slok/goresilience"
    "github.com/slok/goresilience/circuitbreaker"
    "github.com/slok/goresilience/retry"
    "github.com/slok/goresilience/timeout"
    "github.com/slok/goresilience/runnerchain"

    "github.com/compozy/compozy/engine/memory/core"
    "github.com/compozy/compozy/pkg/logger"
)

type ResilientManager struct {
    *Manager // Embed existing manager
    runner   goresilience.Runner
    config   *ResilienceConfig
}

type ResilienceConfig struct {
    TimeoutDuration              time.Duration
    ErrorPercentThresholdToOpen  int
    MinimumRequestToOpen         int
    WaitDurationInOpenState      time.Duration
    RetryTimes                   int
    RetryWaitBase                time.Duration
}

func NewResilientManager(config *ResilienceConfig) *ResilientManager {
    baseManager := NewManager()

    // Create resilience runner with chained middleware
    runner := runnerchain.New(
        // Timeout middleware - fail fast on slow operations
        timeout.NewMiddleware(timeout.Config{
            Timeout: config.TimeoutDuration,
        }),

        // Circuit breaker - fail fast when error rate is high
        circuitbreaker.NewMiddleware(circuitbreaker.Config{
            ErrorPercentThresholdToOpen:        config.ErrorPercentThresholdToOpen,
            MinimumRequestToOpen:               config.MinimumRequestToOpen,
            SuccessfulRequiredOnHalfOpen:       1,
            WaitDurationInOpenState:            config.WaitDurationInOpenState,
            MetricsSlidingWindowBuckets:        10,
            MetricsBucketDuration:              1 * time.Second,
        }),

        // Retry middleware - retry transient failures
        retry.NewMiddleware(retry.Config{
            Times: config.RetryTimes,
            WaitBase: config.RetryWaitBase,
        }),
    )

    return &ResilientManager{
        Manager: baseManager,
        runner:  runner,
        config:  config,
    }
}

// Enhanced redaction with resilience patterns
func (rm *ResilientManager) ApplyPrivacyControlsResilient(
    ctx context.Context,
    msg llm.Message,
    resourceID string,
    metadata core.PrivacyMetadata,
) (llm.Message, core.PrivacyMetadata, error) {

    var result struct {
        message  llm.Message
        metadata core.PrivacyMetadata
        err      error
    }

    // Execute with resilience patterns
    err := rm.runner.Run(ctx, func(ctx context.Context) error {
        // Call original method with resilience wrapper
        msg, meta, err := rm.Manager.ApplyPrivacyControls(ctx, msg, resourceID, metadata)
        result.message = msg
        result.metadata = meta
        result.err = err
        return err
    })

    if err != nil {
        // Resilience patterns failed - use fallback
        return rm.handleResilienceFailure(ctx, msg, resourceID, metadata, err)
    }

    return result.message, result.metadata, result.err
}

func (rm *ResilientManager) handleResilienceFailure(
    ctx context.Context,
    msg llm.Message,
    resourceID string,
    metadata core.PrivacyMetadata,
    resilienceErr error,
) (llm.Message, core.PrivacyMetadata, error) {

    log := logger.FromContext(ctx)
    log.Error("Privacy controls failed with resilience patterns",
        "resource_id", resourceID,
        "resilience_error", resilienceErr,
        "fallback", "no_redaction")

    // Fallback: pass through without redaction but mark metadata
    metadata.RedactionApplied = false
    metadata.DoNotPersist = true // Safe fallback - don't persist potentially sensitive data

    return msg, metadata, nil
}
```

### Configuration Integration

```go
// engine/memory/privacy/config.go
package privacy

import "time"

func DefaultResilienceConfig() *ResilienceConfig {
    return &ResilienceConfig{
        TimeoutDuration:              100 * time.Millisecond,
        ErrorPercentThresholdToOpen:  50, // Open circuit at 50% error rate
        MinimumRequestToOpen:         10, // Need at least 10 requests to open
        WaitDurationInOpenState:      5 * time.Second,
        RetryTimes:                   3,
        RetryWaitBase:                50 * time.Millisecond,
    }
}

func (rm *ResilientManager) UpdateConfig(config *ResilienceConfig) {
    rm.config = config
    // Note: Would need to recreate runner for config changes
    // In practice, these would be set at startup
}
```

### Metrics Integration

```go
// engine/memory/privacy/metrics.go
package privacy

import (
    "context"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    privacyOperationDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "memory_privacy_operation_duration_seconds",
            Help: "Duration of privacy operations",
        },
        []string{"resource_id", "operation", "status"},
    )

    privacyCircuitBreakerState = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "memory_privacy_circuit_breaker_state",
            Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
        },
        []string{"resource_id"},
    )
)

func (rm *ResilientManager) recordMetrics(ctx context.Context, resourceID, operation, status string, duration time.Duration) {
    privacyOperationDuration.WithLabelValues(resourceID, operation, status).Observe(duration.Seconds())
}
```

**Key Implementation Notes:**

- Replaces ~100 lines of custom circuit breaker with proven library
- Modular middleware chaining for timeout, circuit breaker, and retry
- Built-in Prometheus metrics integration
- Graceful fallback when resilience patterns fail
- Configurable parameters for different environments

**⚠️ COMPLEXITY WARNING**: While library integration is straightforward, tuning resilience patterns is complex:

- Requires extensive performance testing under various load conditions
- Finding optimal thresholds for timeouts and failure rates needs empirical testing
- Default library settings will NOT be optimal for production workloads
- Load testing scenarios must be designed to validate configuration under stress

## Success Criteria

- ✅ Custom circuit breaker replaced with goresilience implementation
- ✅ Timeout, retry, and circuit breaker middleware properly configured
- ✅ Prometheus metrics integrated and working
- ✅ Graceful fallback behavior when resilience patterns fail
- ✅ Configuration system allows tuning resilience parameters
- ✅ Comprehensive tests validate resilience behavior under various failure scenarios
- ✅ Performance impact is minimal during normal operations
- ✅ Integration with existing privacy manager maintains compatibility

<critical>
**MANDATORY REQUIREMENTS:**

- **MUST** use `github.com/slok/goresilience` - no custom resilience implementations
- **MUST** maintain backward compatibility with existing privacy manager interface
- **MUST** implement safe fallback behavior when resilience patterns fail
- **MUST** integrate Prometheus metrics for monitoring
- **MUST** include comprehensive test coverage for failure scenarios
- **MUST** follow established configuration patterns
- **MUST** run `make lint` and `make test` before completion
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for completion
  </critical>
