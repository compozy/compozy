---
title: "Workflow Poll Metrics"
group: "ENGINE_GROUP_1_EXECUTION_CORE"
category: "monitoring"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_1_EXECUTION_CORE_MONITORING.md"
issue_index: "3"
sequence: "3"
---

## Workflow Poll Metrics

**Priority:** 🟡 MEDIUM

**Location:** `engine/workflow/router/execute_sync.go`

**Metrics to Add:**

```yaml
workflow_sync_polls_total:
  type: counter
  labels:
    - workflow_id: string
    - outcome: enum[completed, timeout, error]
  description: "Total poll iterations per workflow sync execution"

workflow_sync_poll_duration_seconds:
  type: histogram
  labels:
    - workflow_id: string
  buckets: [0.1, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300]
  description: "Total time spent polling for workflow completion"
```

**Implementation:**

```go
// In waitForWorkflowCompletion:
func waitForWorkflowCompletion(ctx context.Context, ...) (*workflow.Execution, error) {
    pollStart := time.Now()
    attempt := 0

    defer func() {
        // Record total polling time
        workflowPollDuration.Record(ctx, time.Since(pollStart).Seconds(),
            metric.WithAttributes(attribute.String("workflow_id", workflowID.String())))
    }()

    for {
        attempt++

        exec, err := repo.GetExecution(ctx, execID)

        if exec.IsComplete() {
            workflowPollsTotal.Add(ctx, int64(attempt),
                metric.WithAttributes(
                    attribute.String("workflow_id", workflowID.String()),
                    attribute.String("outcome", "completed")))
            return exec, nil
        }

        if ctx.Err() == context.DeadlineExceeded {
            workflowPollsTotal.Add(ctx, int64(attempt),
                metric.WithAttributes(
                    attribute.String("workflow_id", workflowID.String()),
                    attribute.String("outcome", "timeout")))
            return nil, ctx.Err()
        }

        // ... backoff logic
    }
}
```

**Dashboard Queries:**

```promql
# Average polls per workflow
rate(workflow_sync_polls_total[5m]) /
rate(workflow_sync_poll_duration_seconds_count[5m])

# Polling duration p95
histogram_quantile(0.95,
  rate(workflow_sync_poll_duration_seconds_bucket[5m])
) by (workflow_id)
```

**Effort:** S (1h)

## OpenTelemetry Spans

### Task Execute Sync Span

**Location:** `engine/task/router/exec.go`

```go
func executeTaskSync(c *gin.Context, state *server.State) {
    tracer := otel.Tracer("compozy.task")
    ctx, span := tracer.Start(c.Request.Context(), "task.execute_sync")
    defer span.End()

    // Attach attributes
    span.SetAttributes(
        attribute.String("task_id", taskID.String()),
        attribute.String("component", string(taskState.Component)),
        attribute.Int64("timeout_ms", timeoutMs),
    )

    // ... execution logic

    if err != nil {
        span.RecordError(err)
        span.SetAttributes(attribute.String("error_type", categorizeError(err)))
    }
}
```

### Agent Execute Span

**Location:** `engine/agent/exec/runner.go`

```go
func (r *Runner) Execute(ctx context.Context, ...) (*Result, error) {
    tracer := otel.Tracer("compozy.agent")

    // Preparation span
    ctx, prepSpan := tracer.Start(ctx, "agent.prepare")
    config, err := r.loadConfig(ctx, agentID)
    if err != nil {
        prepSpan.RecordError(err)
        prepSpan.End()
        return nil, err
    }
    prepSpan.SetAttributes(attribute.String("action", config.Action))
    prepSpan.End()

    // Execution span
    ctx, execSpan := tracer.Start(ctx, "agent.execute_sync")
    defer execSpan.End()

    execSpan.SetAttributes(
        attribute.String("agent_id", agentID.String()),
        attribute.String("action", config.Action),
    )

    result, err := r.doExecute(ctx, config)
    if err != nil {
        execSpan.RecordError(err)
    }
    return result, err
}
```

### Workflow Execute Sync Span

**Location:** `engine/workflow/router/execute_sync.go`

```go
func executeWorkflowSync(c *gin.Context, state *server.State) {
    tracer := otel.Tracer("compozy.workflow")
    ctx, span := tracer.Start(c.Request.Context(), "workflow.execute_sync")
    defer span.End()

    span.SetAttributes(
        attribute.String("workflow_id", workflowID.String()),
        attribute.String("exec_id", execID.String()),
        attribute.Int64("timeout_ms", timeoutMs),
    )

    // Child span for polling
    ctx, pollSpan := tracer.Start(ctx, "workflow.wait_completion")

    attempt := 0
    for !isComplete {
        attempt++
        pollSpan.AddEvent("poll_iteration",
            attribute.Int("attempt", attempt),
            attribute.String("state", currentState))
        // ... poll logic
    }

    pollSpan.SetAttributes(attribute.Int("total_iterations", attempt))
    pollSpan.End()

    if err != nil {
        span.RecordError(err)
    }
}
```

### Runtime Tool Execution Span

**Location:** `engine/runtime/bun_manager.go`

```go
func (m *Manager) ExecuteToolWithTimeout(ctx context.Context, ...) error {
    tracer := otel.Tracer("compozy.runtime")
    ctx, span := tracer.Start(ctx, "runtime.execute_tool")
    defer span.End()

    span.SetAttributes(
        attribute.String("tool_id", toolID),
        attribute.Int64("timeout_ms", timeoutMs.Milliseconds()),
        attribute.String("bun_version", m.bunVersion),
        attribute.StringSlice("permissions", permissions),
    )

    // ... execution

    if err != nil {
        span.RecordError(err)
        span.SetAttributes(
            attribute.String("error_kind", categorizeError(err)),
        )
    } else {
        span.SetAttributes(
            attribute.Int("exit_code", 0),
            attribute.Int("output_bytes", len(output)),
        )
    }

    return err
}
```

**Effort:** M (2-3h for all spans)

## Dashboard Layout

### Runtime Tools Dashboard

```
┌─────────────────────────────────────────────┐
│ Runtime Tool Execution Latency (p95)        │
│ [Line chart by tool_id]                     │
└─────────────────────────────────────────────┘

┌──────────────────────┬──────────────────────┐
│ Error Rate by Kind   │ Timeout Rate         │
│ [Stacked area]       │ [Single stat + spark]│
└──────────────────────┴──────────────────────┘

┌──────────────────────┬──────────────────────┐
│ Process Exit Codes   │ Output Size p99      │
│ [Heatmap]            │ [Histogram]          │
└──────────────────────┴──────────────────────┘
```

### Execution Overview Dashboard

```
┌─────────────────────────────────────────────┐
│ Execution Latency by Kind (p50/p95/p99)    │
│ [Multi-line: task, agent, workflow]        │
└─────────────────────────────────────────────┘

┌──────────────────────┬──────────────────────┐
│ Async Started Rate   │ Workflow Poll Time   │
│ [Gauge by kind]      │ [Histogram]          │
└──────────────────────┴──────────────────────┘

┌──────────────────────────────────────────────┐
│ Error Rate by Kind and Status Code          │
│ [Heatmap: kind × status]                    │
└──────────────────────────────────────────────┘
```

## Alert Rules

```yaml
groups:
  - name: runtime_tools
    interval: 30s
    rules:
      - alert: HighToolErrorRate
        expr: |
          rate(runtime_tool_errors_total[5m]) > 0.1
        for: 2m
        annotations:
          summary: "High tool error rate: {{ $value | humanize }} errors/sec"

      - alert: ToolTimeoutSpike
        expr: |
          rate(runtime_tool_timeouts_total[5m]) /
          rate(runtime_tool_execute_seconds_count[5m]) > 0.05
        for: 2m
        annotations:
          summary: "Tool timeout rate above 5%"

      - alert: SlowToolExecution
        expr: |
          histogram_quantile(0.95,
            rate(runtime_tool_execute_seconds_bucket[5m])
          ) > 30
        for: 5m
        annotations:
          summary: "Tool p95 latency above 30s"
```

## Implementation Plan

### Week 1 - Critical Metrics

- [ ] Create runtime/metrics.go
- [ ] Instrument ExecuteToolWithTimeout
- [ ] Add error categorization
- [ ] Deploy and verify metrics appear

### Week 2 - Spans & Additional Metrics

- [ ] Add OpenTelemetry spans to all execution paths
- [ ] Add async started counter
- [ ] Add workflow poll metrics
- [ ] Create dashboards

### Week 3 - Refinement

- [ ] Add alert rules
- [ ] Document runbooks
- [ ] Performance tuning if overhead detected
      **Document Version:** 1.0  
       **Last Updated:** 2025-01-16  
       **Owner:** Observability Team
