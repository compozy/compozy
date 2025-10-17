---
title: "Async Execution Started Counters"
group: "ENGINE_GROUP_1_EXECUTION_CORE"
category: "monitoring"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_1_EXECUTION_CORE_MONITORING.md"
issue_index: "2"
sequence: "2"
---

## Async Execution Started Counters

**Priority:** 🟡 MEDIUM

**Location:** `engine/task/router/exec.go:554–610`, `engine/agent/router/exec.go:428–485`

**Metrics to Add:**

```yaml
http_exec_started_total:
  type: counter
  labels:
    - kind: enum[task, agent]
  description: "Total async execution starts accepted"
```

**Implementation:**

```go
// In task/router/exec.go, after successful ExecuteAsync:
func executeTaskAsync(c *gin.Context, state *server.State) {
    // ... existing logic ...

    execID, err := executor.ExecuteAsync(ctx, ...)
    if err != nil {
        // ... error handling
        return
    }

    // Record acceptance
    state.Monitoring.ExecutionMetrics().RecordAsyncStarted(ctx, "task")

    c.JSON(202, gin.H{"execution_id": execID})
}
```

**Extend ExecutionMetrics:**

```go
// In engine/infra/monitoring/execution_metrics.go:

type ExecutionMetrics struct {
    // ... existing fields
    asyncStartedCounter metric.Int64Counter
}

func newExecutionMetrics(meter metric.Meter) (*ExecutionMetrics, error) {
    // ... existing metrics

    asyncStarted, err := meter.Int64Counter(
        "http_exec_started_total",
        metric.WithDescription("Total async execution starts accepted"),
    )
    if err != nil {
        return nil, err
    }

    return &ExecutionMetrics{
        // ... existing
        asyncStartedCounter: asyncStarted,
    }, nil
}

func (m *ExecutionMetrics) RecordAsyncStarted(ctx context.Context, kind string) {
    if m == nil || m.asyncStartedCounter == nil {
        return
    }
    m.asyncStartedCounter.Add(ctx, 1,
        metric.WithAttributes(attribute.String("kind", kind)))
}
```

**Dashboard Queries:**

```promql
# Async acceptance rate
rate(http_exec_started_total[5m]) by (kind)

# Async vs sync ratio
rate(http_exec_started_total[5m]) /
rate(http_exec_sync_latency_seconds_count[5m])
```

**Effort:** S (1h)
