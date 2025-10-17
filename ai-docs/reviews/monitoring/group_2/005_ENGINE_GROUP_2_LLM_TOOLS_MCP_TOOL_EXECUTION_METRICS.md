---
title: "MCP Tool Execution Metrics"
group: "ENGINE_GROUP_2_LLM_TOOLS"
category: "monitoring"
priority: "🔴 HIGH"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_2_LLM_TOOLS_MONITORING.md"
issue_index: "2"
sequence: "5"
---

## MCP Tool Execution Metrics

**Priority:** 🔴 HIGH

**Location:** `engine/mcp/client/`, `engine/mcp/tools/executor.go`

**Metrics to Add:**

```yaml
mcp_tool_execute_seconds:
  type: histogram
  unit: seconds
  labels:
    - server_id: string
    - tool_name: string
    - outcome: enum[success, error, timeout]
  buckets: [0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30]
  description: "MCP tool execution latency"

mcp_tool_errors_total:
  type: counter
  labels:
    - server_id: string
    - tool_name: string
    - error_kind: enum[connection, validation, execution, timeout]
  description: "MCP tool errors by category"

mcp_server_connections_active:
  type: gauge
  labels:
    - server_id: string
  description: "Active MCP server connections"

mcp_tool_registry_lookup_seconds:
  type: histogram
  unit: seconds
  labels:
    - outcome: enum[hit, miss]
  buckets: [0.00001, 0.0001, 0.001, 0.01, 0.1]
  description: "Tool registry lookup latency"
```

**Implementation:**

```go
// engine/mcp/metrics/metrics.go (NEW)
package metrics

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

    executeLatency    metric.Float64Histogram
    errorsCounter     metric.Int64Counter
    activeConnections metric.Int64UpDownCounter
    registryLookup    metric.Float64Histogram
)

func initMetrics() {
    meter := otel.GetMeterProvider().Meter("compozy.mcp")

    executeLatency, _ = meter.Float64Histogram(
        "mcp_tool_execute_seconds",
        metric.WithDescription("MCP tool execution latency"),
        metric.WithUnit("s"),
        metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30),
    )

    errorsCounter, _ = meter.Int64Counter(
        "mcp_tool_errors_total",
        metric.WithDescription("MCP tool errors by category"),
    )

    activeConnections, _ = meter.Int64UpDownCounter(
        "mcp_server_connections_active",
        metric.WithDescription("Active MCP server connections"),
    )

    registryLookup, _ = meter.Float64Histogram(
        "mcp_tool_registry_lookup_seconds",
        metric.WithDescription("Tool registry lookup latency"),
        metric.WithUnit("s"),
        metric.WithExplicitBucketBoundaries(0.00001, 0.0001, 0.001, 0.01, 0.1),
    )
}

func RecordExecution(ctx context.Context, serverID, toolName string, duration time.Duration, outcome string) {
    once.Do(initMetrics)

    executeLatency.Record(ctx, duration.Seconds(),
        metric.WithAttributes(
            attribute.String("server_id", serverID),
            attribute.String("tool_name", toolName),
            attribute.String("outcome", outcome),
        ))
}

func RecordError(ctx context.Context, serverID, toolName, errorKind string) {
    once.Do(initMetrics)

    errorsCounter.Add(ctx, 1,
        metric.WithAttributes(
            attribute.String("server_id", serverID),
            attribute.String("tool_name", toolName),
            attribute.String("error_kind", errorKind),
        ))
}

func RecordConnection(ctx context.Context, serverID string, delta int) {
    once.Do(initMetrics)

    activeConnections.Add(ctx, int64(delta),
        metric.WithAttributes(attribute.String("server_id", serverID)))
}

func RecordRegistryLookup(ctx context.Context, duration time.Duration, hit bool) {
    once.Do(initMetrics)

    outcome := "miss"
    if hit {
        outcome = "hit"
    }

    registryLookup.Record(ctx, duration.Seconds(),
        metric.WithAttributes(attribute.String("outcome", outcome)))
}
```

**Usage:**

```go
// engine/mcp/tools/executor.go
func (e *Executor) Execute(ctx context.Context, serverID, toolName string, args map[string]any) (*Result, error) {
    start := time.Now()

    result, err := e.client.CallTool(ctx, serverID, toolName, args)

    duration := time.Since(start)
    outcome := "success"

    if err != nil {
        outcome = "error"
        errorKind := categorizeError(err)
        metrics.RecordError(ctx, serverID, toolName, errorKind)
    }

    metrics.RecordExecution(ctx, serverID, toolName, duration, outcome)
    return result, err
}

// engine/mcp/registry/registry.go
func (r *Registry) FindTool(ctx context.Context, toolName string) (*Tool, error) {
    start := time.Now()

    // Check cache
    tool, hit := r.toolIndex.Load(toolName)

    metrics.RecordRegistryLookup(ctx, time.Since(start), hit)

    if hit {
        return tool.(*Tool), nil
    }

    // ... load from store
}

// engine/mcp/client/client.go
func (c *Client) connect(ctx context.Context, serverID string) (*Connection, error) {
    conn, err := c.dial(ctx, serverID)
    if err != nil {
        return nil, err
    }

    metrics.RecordConnection(ctx, serverID, 1)

    // Track disconnection
    go func() {
        <-conn.Done()
        metrics.RecordConnection(context.Background(), serverID, -1)
    }()

    return conn, nil
}
```

**Dashboard Queries:**

```promql
# MCP tool latency p95 by server
histogram_quantile(0.95,
  rate(mcp_tool_execute_seconds_bucket[5m])
) by (server_id, tool_name)

# Error rate by tool
rate(mcp_tool_errors_total[5m]) by (tool_name, error_kind)

# Registry lookup performance
histogram_quantile(0.99,
  rate(mcp_tool_registry_lookup_seconds_bucket[5m])
) by (outcome)

# Active connections per server
mcp_server_connections_active by (server_id)
```

**Effort:** M (3h)
