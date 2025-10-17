---
title: "Dispatcher Health Metric Cardinality Explosion"
group: "ENGINE_GROUP_4_INFRASTRUCTURE"
category: "performance"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_4_INFRASTRUCTURE_PERFORMANCE.md"
issue_index: "1"
sequence: "17"
---

## Dispatcher Health Metric Cardinality Explosion

**Location:** `engine/infra/monitoring/dispatcher.go:96–121`

**Severity:** 🔴 CRITICAL

**Issue:**

```go
// Lines 96-121 - WRONG: Time-varying labels create unbounded cardinality
dispatcherHealthCallback, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
    now := time.Now()
    dispatcherHealthStore.Range(func(key, value any) bool {
        // ... get dispatcher health ...
        healthValue, isStale, timeSinceHeartbeat, failures := health.getMetricValues(now)
        o.ObserveInt64(dispatcherHealthGauge, healthValue,
            metric.WithAttributes(
                attribute.String("dispatcher_id", dispatcherID),
                attribute.Bool("is_stale", isStale),
                attribute.Float64("time_since_heartbeat", timeSinceHeartbeat), // ❌ CHANGES EVERY OBSERVATION
                attribute.Int64("consecutive_failures", int64(failures)),      // ❌ CHANGES FREQUENTLY
            ))
        return true
    })
    return nil
}, dispatcherHealthGauge)
```

**Problems:**

1. `time_since_heartbeat` changes every observation creating new time series
2. `consecutive_failures` creates new series for every failure count
3. With N dispatchers and M observations/minute, creates N _ M _ T unique time series
4. Prometheus has 10M series limit by default - this blows it up rapidly
5. Query performance degrades exponentially with cardinality

**Impact:**

- **Memory:** 1KB per unique series → 1GB for 1M series
- **Query latency:** 10ms → 10s+ for high cardinality
- **Storage:** 1GB/day → 100GB/day for 100K series
- **Production outage:** Prometheus crashes when limit exceeded

**Fix:**

```go
// engine/infra/monitoring/dispatcher.go
var (
    dispatcherHealthGauge          metric.Int64ObservableGauge
    dispatcherHeartbeatAgeSeconds  metric.Float64ObservableGauge  // NEW: Separate metric for age
    dispatcherFailureCount         metric.Int64ObservableGauge    // NEW: Separate metric for failures
    dispatcherHealthCallback       metric.Registration
)

func initDispatcherHealthMetrics(ctx context.Context, meter metric.Meter) {
    // ... existing init code ...

    // Health status: only dispatcher_id and is_stale as labels
    dispatcherHealthGauge, err = meter.Int64ObservableGauge(
        "compozy_dispatcher_health_status",
        metric.WithDescription("Dispatcher health status (1=healthy, 0=unhealthy)"),
    )

    // Heartbeat age: separate metric without cardinality issues
    dispatcherHeartbeatAgeSeconds, err = meter.Float64ObservableGauge(
        "compozy_dispatcher_heartbeat_age_seconds",
        metric.WithDescription("Seconds since last dispatcher heartbeat"),
    )

    // Failure count: separate gauge metric
    dispatcherFailureCount, err = meter.Int64ObservableGauge(
        "compozy_dispatcher_consecutive_failures",
        metric.WithDescription("Number of consecutive health check failures"),
    )

    // Single callback for all three metrics
    dispatcherHealthCallback, err = meter.RegisterCallback(
        func(_ context.Context, o metric.Observer) error {
            now := time.Now()
            dispatcherHealthStore.Range(func(key, value any) bool {
                dispatcherID, ok := key.(string)
                if !ok {
                    return true
                }
                health, ok := value.(*DispatcherHealth)
                if !ok {
                    return true
                }

                health.UpdateHealth()
                healthValue, isStale, timeSinceHeartbeat, failures := health.getMetricValues(now)

                // Observe health status with minimal labels
                o.ObserveInt64(dispatcherHealthGauge, healthValue,
                    metric.WithAttributes(
                        attribute.String("dispatcher_id", dispatcherID),
                        attribute.Bool("is_stale", isStale),
                    ))

                // Observe heartbeat age as value, not label
                o.ObserveFloat64(dispatcherHeartbeatAgeSeconds, timeSinceHeartbeat,
                    metric.WithAttributes(
                        attribute.String("dispatcher_id", dispatcherID),
                    ))

                // Observe failure count as value, not label
                o.ObserveInt64(dispatcherFailureCount, int64(failures),
                    metric.WithAttributes(
                        attribute.String("dispatcher_id", dispatcherID),
                    ))

                return true
            })
            return nil
        },
        dispatcherHealthGauge,
        dispatcherHeartbeatAgeSeconds,
        dispatcherFailureCount,
    )
}
```

**Queries After Fix:**

```promql
# Stale dispatchers (unchanged)
sum(compozy_dispatcher_health_status{is_stale="true"})

# Average heartbeat age (now works!)
avg(compozy_dispatcher_heartbeat_age_seconds)

# Dispatchers with >3 consecutive failures
count(compozy_dispatcher_consecutive_failures > 3)

# Heartbeat age by dispatcher
topk(10, compozy_dispatcher_heartbeat_age_seconds)
```

**Cardinality Comparison:**

- **Before:** N dispatchers _ O observations/min _ F failure states \* T seconds = unbounded
- **After:** N dispatchers \* 3 metrics = 3N series (bounded)
- **Savings:** 99.9% reduction (1M series → 300 series for 100 dispatchers)

**Effort:** M (4h)  
**Risk:** Low - metrics improvement, no behavior change
