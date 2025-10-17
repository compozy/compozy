---
title: "Tool Factory & Provider Factory Metrics"
group: "ENGINE_GROUP_2_LLM_TOOLS"
category: "monitoring"
priority: "🟢 LOW"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_2_LLM_TOOLS_MONITORING.md"
issue_index: "4"
sequence: "7"
---

## Tool Factory & Provider Factory Metrics

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
      **Document Version:** 1.0  
       **Last Updated:** 2025-01-16  
       **Owner:** Observability Team
