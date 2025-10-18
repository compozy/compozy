# Engine Group 2: LLM & Tools - Monitoring Improvements

**Packages:** llm (provider, factory, pooling), mcp (client, server, registry, tools), tool (runtime, execution)

---

## Executive Summary

Comprehensive monitoring for LLM provider interactions, MCP tool executions, and tool registry operations. Critical gaps exist in provider call instrumentation and tool execution observability.

**Current State:**

- ✅ Basic HTTP metrics exist
- ❌ No provider-specific call metrics (tokens, latency, errors)
- ❌ No MCP tool execution metrics
- ❌ No tool registry lookup performance tracking
- ❌ No usage persistence metrics

---

## Missing Metrics

### 1. LLM Provider Call Metrics

**Priority:** 🔴 CRITICAL

**Location:** `engine/llm/provider/`

**Why Critical:**

- Provider calls are most expensive operations (cost & latency)
- No visibility into token usage, rate limits, or failures
- Cannot optimize model selection or detect cost anomalies

**Metrics to Add:**

```yaml
llm_provider_request_seconds:
  type: histogram
  unit: seconds
  labels:
    - provider: string (openai, anthropic, gemini)
    - model: string (gpt-4, claude-3, etc.)
    - outcome: enum[success, error, timeout, rate_limited]
  buckets: [0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120]
  description: "Provider API call latency"

llm_provider_tokens_total:
  type: counter
  labels:
    - provider: string
    - model: string
    - type: enum[prompt, completion]
  description: "Total tokens consumed by provider and type"

llm_provider_cost_usd_total:
  type: counter
  labels:
    - provider: string
    - model: string
  description: "Estimated cumulative cost in USD"

llm_provider_errors_total:
  type: counter
  labels:
    - provider: string
    - model: string
    - error_type: enum[auth, rate_limit, invalid_request, server_error, timeout]
  description: "Provider errors by category"

llm_provider_rate_limit_delays_seconds:
  type: histogram
  labels:
    - provider: string
  buckets: [0.1, 0.5, 1, 2, 5, 10, 30, 60]
  description: "Duration spent waiting due to rate limits"
```

**Implementation:**

```go
// engine/llm/metrics/metrics.go (NEW FILE)
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

    requestLatency     metric.Float64Histogram
    tokensCounter      metric.Int64Counter
    costCounter        metric.Float64Counter
    errorsCounter      metric.Int64Counter
    rateLimitDelays    metric.Float64Histogram
)

func initMetrics() {
    meter := otel.GetMeterProvider().Meter("compozy.llm")

    requestLatency, _ = meter.Float64Histogram(
        "llm_provider_request_seconds",
        metric.WithDescription("Provider API call latency"),
        metric.WithUnit("s"),
        metric.WithExplicitBucketBoundaries(0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120),
    )

    tokensCounter, _ = meter.Int64Counter(
        "llm_provider_tokens_total",
        metric.WithDescription("Total tokens consumed"),
    )

    costCounter, _ = meter.Float64Counter(
        "llm_provider_cost_usd_total",
        metric.WithDescription("Estimated cost in USD"),
    )

    errorsCounter, _ = meter.Int64Counter(
        "llm_provider_errors_total",
        metric.WithDescription("Provider errors by category"),
    )

    rateLimitDelays, _ = meter.Float64Histogram(
        "llm_provider_rate_limit_delays_seconds",
        metric.WithDescription("Rate limit wait time"),
        metric.WithUnit("s"),
        metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 2, 5, 10, 30, 60),
    )
}

func RecordRequest(ctx context.Context, provider, model string, duration time.Duration, outcome string) {
    once.Do(initMetrics)

    requestLatency.Record(ctx, duration.Seconds(),
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("model", model),
            attribute.String("outcome", outcome),
        ))
}

func RecordTokens(ctx context.Context, provider, model, tokenType string, count int) {
    once.Do(initMetrics)

    tokensCounter.Add(ctx, int64(count),
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("model", model),
            attribute.String("type", tokenType),
        ))
}

func RecordCost(ctx context.Context, provider, model string, costUSD float64) {
    once.Do(initMetrics)

    costCounter.Add(ctx, costUSD,
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("model", model),
        ))
}

func RecordError(ctx context.Context, provider, model, errorType string) {
    once.Do(initMetrics)

    errorsCounter.Add(ctx, 1,
        metric.WithAttributes(
            attribute.String("provider", provider),
            attribute.String("model", model),
            attribute.String("error_type", errorType),
        ))
}

func RecordRateLimitDelay(ctx context.Context, provider string, delay time.Duration) {
    once.Do(initMetrics)

    rateLimitDelays.Record(ctx, delay.Seconds(),
        metric.WithAttributes(attribute.String("provider", provider)))
}
```

**Usage in Provider:**

```go
// engine/llm/provider/openai.go
func (p *OpenAIProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    start := time.Now()

    resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    req.Model,
        Messages: convertMessages(req.Messages),
    })

    duration := time.Since(start)

    if err != nil {
        outcome := categorizeError(err)
        metrics.RecordRequest(ctx, "openai", req.Model, duration, "error")
        metrics.RecordError(ctx, "openai", req.Model, outcome)
        return nil, err
    }

    // Record success metrics
    metrics.RecordRequest(ctx, "openai", req.Model, duration, "success")

    // Record token usage
    metrics.RecordTokens(ctx, "openai", req.Model, "prompt", resp.Usage.PromptTokens)
    metrics.RecordTokens(ctx, "openai", req.Model, "completion", resp.Usage.CompletionTokens)

    // Record cost
    cost := calculateCost(req.Model, resp.Usage)
    metrics.RecordCost(ctx, "openai", req.Model, cost)

    return &CompletionResponse{
        Content: resp.Choices[0].Message.Content,
        Usage: &UsageMetadata{
            PromptTokens:     resp.Usage.PromptTokens,
            CompletionTokens: resp.Usage.CompletionTokens,
            TotalTokens:      resp.Usage.TotalTokens,
        },
    }, nil
}

func categorizeError(err error) string {
    if errors.Is(err, context.DeadlineExceeded) {
        return "timeout"
    }

    // OpenAI-specific error types
    var apiErr *openai.APIError
    if errors.As(err, &apiErr) {
        switch apiErr.HTTPStatusCode {
        case 401, 403:
            return "auth"
        case 429:
            return "rate_limit"
        case 400:
            return "invalid_request"
        case 500, 502, 503, 504:
            return "server_error"
        }
    }

    return "unknown"
}
```

**Dashboard Queries:**

```promql
# Provider latency by model
histogram_quantile(0.95,
  rate(llm_provider_request_seconds_bucket[5m])
) by (provider, model)

# Token consumption rate
rate(llm_provider_tokens_total[5m]) by (provider, model, type)

# Cost per minute
rate(llm_provider_cost_usd_total[1m]) * 60

# Error rate by type
rate(llm_provider_errors_total[5m]) by (error_type)

# Rate limit frequency
rate(llm_provider_errors_total{error_type="rate_limit"}[5m])
```

**Effort:** M (3-4h)

---

### 2. MCP Tool Execution Metrics

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

---

### 3. Usage Persistence Metrics

**Priority:** 🟡 MEDIUM

**Location:** `engine/llm/usage/repository.go`

**Metrics to Add:**

```yaml
llm_usage_persist_seconds:
  type: histogram
  unit: seconds
  labels:
    - outcome: enum[success, error]
  buckets: [0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1]
  description: "Usage record persistence latency"

llm_usage_persist_errors_total:
  type: counter
  labels:
    - error_type: enum[db_error, timeout, validation]
  description: "Usage persistence failures"

llm_usage_persist_queue_size:
  type: gauge
  description: "Pending usage records waiting to be persisted"
```

**Implementation:**

```go
// engine/llm/usage/metrics.go (NEW)
package usage

import (
    "context"
    "sync"
    "time"
    "go.opentelemetry.io/otel/metric"
)

var (
    once sync.Once

    persistLatency metric.Float64Histogram
    persistErrors  metric.Int64Counter
    queueSize      metric.Int64UpDownCounter
)

func initMetrics() {
    meter := otel.GetMeterProvider().Meter("compozy.llm.usage")

    persistLatency, _ = meter.Float64Histogram(
        "llm_usage_persist_seconds",
        metric.WithDescription("Usage persistence latency"),
        metric.WithUnit("s"),
    )

    persistErrors, _ = meter.Int64Counter(
        "llm_usage_persist_errors_total",
        metric.WithDescription("Persistence failures"),
    )

    queueSize, _ = meter.Int64UpDownCounter(
        "llm_usage_persist_queue_size",
        metric.WithDescription("Pending usage records"),
    )
}

func RecordPersist(ctx context.Context, duration time.Duration, err error) {
    once.Do(initMetrics)

    outcome := "success"
    if err != nil {
        outcome = "error"
    }

    persistLatency.Record(ctx, duration.Seconds(),
        metric.WithAttributes(attribute.String("outcome", outcome)))

    if err != nil {
        errorType := categorizeError(err)
        persistErrors.Add(ctx, 1,
            metric.WithAttributes(attribute.String("error_type", errorType)))
    }
}

func IncrementQueueSize(ctx context.Context) {
    once.Do(initMetrics)
    queueSize.Add(ctx, 1)
}

func DecrementQueueSize(ctx context.Context) {
    once.Do(initMetrics)
    queueSize.Add(ctx, -1)
}
```

**Dashboard Queries:**

```promql
# Persistence latency p99
histogram_quantile(0.99,
  rate(llm_usage_persist_seconds_bucket[5m])
)

# Error rate
rate(llm_usage_persist_errors_total[5m]) by (error_type)

# Queue backlog
llm_usage_persist_queue_size
```

**Effort:** S (1-2h)

---

### 4. Tool Factory & Provider Factory Metrics

**Priority:** 🟢 LOW

**Location:** `engine/llm/factory/`, `engine/mcp/registry/factory.go`

**Metrics to Add:**

```yaml
factory_create_seconds:
  type: histogram
  labels:
    - factory_type: enum[provider, tool]
    - name: string
  buckets: [0.00001, 0.0001, 0.001, 0.01]
  description: "Factory instantiation time"
```

**Effort:** S (1h)

---

## OpenTelemetry Spans

### Provider Completion Span

```go
func (p *Provider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    tracer := otel.Tracer("compozy.llm")
    ctx, span := tracer.Start(ctx, "llm.complete")
    defer span.End()

    span.SetAttributes(
        attribute.String("provider", p.name),
        attribute.String("model", req.Model),
        attribute.Int("prompt_length", len(req.Messages)),
    )

    resp, err := p.doComplete(ctx, req)

    if err != nil {
        span.RecordError(err)
        span.SetAttributes(attribute.String("error_type", categorizeError(err)))
    } else {
        span.SetAttributes(
            attribute.Int("prompt_tokens", resp.Usage.PromptTokens),
            attribute.Int("completion_tokens", resp.Usage.CompletionTokens),
        )
    }

    return resp, err
}
```

### MCP Tool Execution Span

```go
func (e *Executor) Execute(ctx context.Context, serverID, toolName string, args map[string]any) (*Result, error) {
    tracer := otel.Tracer("compozy.mcp")
    ctx, span := tracer.Start(ctx, "mcp.tool.execute")
    defer span.End()

    span.SetAttributes(
        attribute.String("server_id", serverID),
        attribute.String("tool_name", toolName),
        attribute.Int("arg_count", len(args)),
    )

    result, err := e.client.CallTool(ctx, serverID, toolName, args)

    if err != nil {
        span.RecordError(err)
    }

    return result, err
}
```

**Effort:** M (2h for all spans)

---

## Dashboard Layout

### LLM Provider Dashboard

```
┌─────────────────────────────────────────────┐
│ Provider Request Latency (p95) by Model    │
│ [Multi-line: GPT-4, Claude-3, etc.]        │
└─────────────────────────────────────────────┘

┌──────────────────────┬──────────────────────┐
│ Token Usage Rate     │ Estimated Cost/Hour  │
│ [Stacked area]       │ [$XX.XX + trend]     │
└──────────────────────┴──────────────────────┘

┌──────────────────────┬──────────────────────┐
│ Error Rate by Type   │ Rate Limit Events    │
│ [Stacked bar]        │ [Event timeline]     │
└──────────────────────┴──────────────────────┘
```

### MCP Tools Dashboard

```
┌─────────────────────────────────────────────┐
│ Tool Execution Latency p95 by Server       │
│ [Heatmap: server_id × tool_name]           │
└─────────────────────────────────────────────┘

┌──────────────────────┬──────────────────────┐
│ Active Connections   │ Registry Lookup      │
│ [Gauge by server]    │ [Histogram: hit/miss]│
└──────────────────────┴──────────────────────┘

┌──────────────────────────────────────────────┐
│ Error Rate by Tool and Kind                 │
│ [Stacked area]                              │
└──────────────────────────────────────────────┘
```

---

## Alert Rules

```yaml
groups:
  - name: llm_providers
    interval: 30s
    rules:
      - alert: HighProviderErrorRate
        expr: |
          rate(llm_provider_errors_total[5m]) > 0.1
        for: 2m
        annotations:
          summary: "Provider {{ $labels.provider }} error rate: {{ $value | humanize }}/sec"

      - alert: ExcessiveTokenUsage
        expr: |
          rate(llm_provider_tokens_total[5m]) > 50000
        for: 5m
        annotations:
          summary: "Token consumption spike: {{ $value | humanize }} tokens/sec"

      - alert: HighProviderCost
        expr: |
          rate(llm_provider_cost_usd_total[1m]) * 3600 > 100
        for: 5m
        annotations:
          summary: "Estimated cost exceeds $100/hour"

  - name: mcp_tools
    interval: 30s
    rules:
      - alert: HighMCPToolErrorRate
        expr: |
          rate(mcp_tool_errors_total[5m]) > 0.05
        for: 2m
        annotations:
          summary: "MCP tool {{ $labels.tool_name }} error rate high"

      - alert: SlowToolExecution
        expr: |
          histogram_quantile(0.95,
            rate(mcp_tool_execute_seconds_bucket[5m])
          ) > 10
        for: 5m
        annotations:
          summary: "Tool execution p95 > 10s"
```

---

## Implementation Plan

### Week 1 - Provider Metrics

- [ ] Create llm/metrics package
- [ ] Instrument all providers (OpenAI, Anthropic, Gemini)
- [ ] Add error categorization
- [ ] Deploy and verify

### Week 2 - MCP Metrics

- [ ] Create mcp/metrics package
- [ ] Instrument tool executor
- [ ] Add registry lookup tracking
- [ ] Add connection tracking

### Week 3 - Dashboards & Alerts

- [ ] Create Grafana dashboards
- [ ] Configure alert rules
- [ ] Document runbooks
- [ ] Add usage persistence metrics

---

**Document Version:** 1.0  
**Last Updated:** 2025-01-16  
**Owner:** Observability Team
