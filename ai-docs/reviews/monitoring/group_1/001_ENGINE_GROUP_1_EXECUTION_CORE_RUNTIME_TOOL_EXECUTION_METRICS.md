---
title: "Runtime Tool Execution Metrics"
group: "ENGINE_GROUP_1_EXECUTION_CORE"
category: "monitoring"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_1_EXECUTION_CORE_MONITORING.md"
issue_index: "1"
sequence: "1"
---

## Runtime Tool Execution Metrics

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
