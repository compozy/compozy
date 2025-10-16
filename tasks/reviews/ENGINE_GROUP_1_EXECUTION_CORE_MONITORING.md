# Engine Group 1: Execution Core - Monitoring Improvements

**Packages:** agent, task, task2, workflow, runtime

---

## Executive Summary

Comprehensive monitoring instrumentation for execution core components to provide visibility into tool execution, workflow polling, task/agent execution, and runtime performance.

**Current State:**

- ✅ ExecutionMetrics exist (RecordSyncLatency, RecordTimeout, RecordError)
- ❌ No runtime tool execution metrics
- ❌ No OpenTelemetry spans for execution paths
- ❌ No workflow polling metrics
- ❌ No async execution started counters

---

## Missing Metrics

### 1. Runtime Tool Execution Metrics

**Priority:** 🔴 CRITICAL

**Location:** `engine/runtime/bun_manager.go`

**Why Critical:**

- Runtime tool execution is black box today
- Cannot debug tool failures, timeouts, or performance issues
- No visibility into process exit codes or error types

**Metrics to Add:**

```yaml
runtime_tool_execute_seconds:
  type: histogram
  unit: seconds
  labels:
    - tool_id: string (tool identifier)
    - outcome: enum[success, error, timeout]
  buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30]
  description: "Latency of runtime tool executions from start to completion"

runtime_tool_errors_total:
  type: counter
  labels:
    - tool_id: string
    - error_kind: enum[start, stdin, stdout, stderr, wait, parse, timeout]
  description: "Total runtime tool errors categorized by failure point"

runtime_tool_timeouts_total:
  type: counter
  labels:
    - tool_id: string
  description: "Total tool executions that exceeded timeout"

runtime_bun_process_exits_total:
  type: counter
  labels:
    - status: enum[exit, signal]
    - code: int (exit code, 0-255)
    - signal: string (e.g., SIGKILL, SIGTERM)
  description: "Bun process termination reasons"

runtime_tool_output_bytes:
  type: histogram
  labels:
    - tool_id: string
  buckets: [100, 1000, 10000, 100000, 1000000, 10000000]
  description: "Size distribution of tool stdout payloads"
```

**Implementation:**

```go
// engine/runtime/metrics.go (NEW FILE)
package runtime

import (
    "context"
    "sync"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    executeLatency metric.Float64Histogram
    errorCounter   metric.Int64Counter
    timeoutCounter metric.Int64Counter
    processExits   metric.Int64Counter
    outputBytes    metric.Float64Histogram
)

func initMetrics() {
    meter := otel.GetMeterProvider().Meter("compozy.runtime")

    executeLatency, _ = meter.Float64Histogram(
        "runtime_tool_execute_seconds",
        metric.WithDescription("Latency of runtime tool executions"),
        metric.WithUnit("s"),
        metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30),
    )

    errorCounter, _ = meter.Int64Counter(
        "runtime_tool_errors_total",
        metric.WithDescription("Total runtime tool errors by kind"),
    )

    timeoutCounter, _ = meter.Int64Counter(
        "runtime_tool_timeouts_total",
        metric.WithDescription("Total tool timeouts"),
    )

    processExits, _ = meter.Int64Counter(
        "runtime_bun_process_exits_total",
        metric.WithDescription("Bun process exit reasons"),
    )

    outputBytes, _ = meter.Float64Histogram(
        "runtime_tool_output_bytes",
        metric.WithDescription("Tool output payload sizes"),
        metric.WithExplicitBucketBoundaries(100, 1000, 10000, 100000, 1000000, 10000000),
    )
}

func RecordExecution(ctx context.Context, toolID string, duration time.Duration, outcome string) {
    once.Do(initMetrics)

    executeLatency.Record(ctx, duration.Seconds(),
        metric.WithAttributes(
            attribute.String("tool_id", toolID),
            attribute.String("outcome", outcome),
        ))
}

func RecordError(ctx context.Context, toolID string, errorKind string) {
    once.Do(initMetrics)

    errorCounter.Add(ctx, 1,
        metric.WithAttributes(
            attribute.String("tool_id", toolID),
            attribute.String("error_kind", errorKind),
        ))
}

func RecordTimeout(ctx context.Context, toolID string) {
    once.Do(initMetrics)

    timeoutCounter.Add(ctx, 1,
        metric.WithAttributes(attribute.String("tool_id", toolID)))
}

func RecordProcessExit(ctx context.Context, exitCode int, signal string) {
    once.Do(initMetrics)

    status := "exit"
    if signal != "" {
        status = "signal"
    }

    processExits.Add(ctx, 1,
        metric.WithAttributes(
            attribute.String("status", status),
            attribute.Int("code", exitCode),
            attribute.String("signal", signal),
        ))
}

func RecordOutputSize(ctx context.Context, toolID string, bytes int) {
    once.Do(initMetrics)

    outputBytes.Record(ctx, float64(bytes),
        metric.WithAttributes(attribute.String("tool_id", toolID)))
}
```

**Usage in bun_manager.go:**

```go
// In ExecuteToolWithTimeout:
func (m *Manager) ExecuteToolWithTimeout(ctx context.Context, toolID string, ...) error {
    start := time.Now()

    // ... existing execution logic ...

    // Record metrics
    outcome := "success"
    if err != nil {
        outcome = "error"

        // Categorize error
        switch {
        case errors.Is(err, context.DeadlineExceeded):
            runtime.RecordTimeout(ctx, toolID)
            outcome = "timeout"
        case isStartError(err):
            runtime.RecordError(ctx, toolID, "start")
        case isStdinError(err):
            runtime.RecordError(ctx, toolID, "stdin")
        case isParseError(err):
            runtime.RecordError(ctx, toolID, "parse")
        default:
            runtime.RecordError(ctx, toolID, "wait")
        }
    }

    runtime.RecordExecution(ctx, toolID, time.Since(start), outcome)
    return err
}

// In waitForProcessCompletion:
func waitForProcessCompletion(ctx context.Context, cmd *exec.Cmd) error {
    waitErr := cmd.Wait()

    if waitErr != nil {
        if exitErr, ok := waitErr.(*exec.ExitError); ok {
            runtime.RecordProcessExit(ctx, exitErr.ExitCode(), "")
        }
    } else {
        runtime.RecordProcessExit(ctx, 0, "")
    }

    // ... error mapping
}

// In readStdoutResponse:
func readStdoutResponse(ctx context.Context, toolID string, stdout io.Reader) ([]byte, error) {
    data, err := io.ReadAll(stdout)
    if err == nil {
        runtime.RecordOutputSize(ctx, toolID, len(data))
    }
    return data, err
}
```

**Dashboard Queries:**

```promql
# Tool execution latency p95
histogram_quantile(0.95,
  rate(runtime_tool_execute_seconds_bucket[5m])
) by (tool_id)

# Tool error rate
rate(runtime_tool_errors_total[5m]) by (error_kind)

# Tool timeout rate
rate(runtime_tool_timeouts_total[5m]) /
rate(runtime_tool_execute_seconds_count[5m])

# Process exit codes distribution
rate(runtime_bun_process_exits_total[5m]) by (code)
```

**Effort:** M (2-3h)

---

### 2. Async Execution Started Counters

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

---

### 3. Workflow Poll Metrics

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

---

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

---

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

---

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

---

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

---

**Document Version:** 1.0  
**Last Updated:** 2025-01-16  
**Owner:** Observability Team
