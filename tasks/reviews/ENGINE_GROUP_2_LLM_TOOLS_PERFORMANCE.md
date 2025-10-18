# Engine Group 2: LLM & Tools - Performance Improvements

**Packages:** llm (provider, factory, pooling), mcp (client, server, registry, tools), tool (runtime, execution)

---

## Executive Summary

Performance optimization opportunities in LLM provider abstraction, MCP tool integration, and tool execution infrastructure. Focus on reducing latency, improving connection pooling, and optimizing lookup operations.

**Priority Findings:**

- 🔴 **Critical:** O(n) linear scan for MCP tool lookup under load
- 🔴 **High Impact:** No usage metadata persistence causing data loss
- 🟡 **Medium Impact:** Missing provider call result caching
- 🟡 **Medium Impact:** No connection pooling for streaming responses

---

## High Priority Issues

### 1. O(n) MCP Tool Lookup Performance

**Location:** `engine/mcp/registry/lookup.go:45–89`, `engine/mcp/tools/resolver.go:112–156`

**Severity:** 🔴 CRITICAL

**Issue:**

```go
// Linear scan through all registered MCP tools
func (r *Registry) FindTool(ctx context.Context, toolName string) (*Tool, error) {
    tools, err := r.ListAll(ctx)
    if err != nil {
        return nil, err
    }

    // O(n) scan - slow when 100+ tools registered
    for _, tool := range tools {
        if tool.Name == toolName {
            return tool, nil
        }
    }

    return nil, ErrToolNotFound
}

// Called on EVERY tool invocation during agent execution
func (e *Executor) resolveTool(ctx context.Context, name string) (*ToolDefinition, error) {
    // Hits O(n) lookup every time
    tool, err := e.registry.FindTool(ctx, name)
    // ...
}
```

**Problems:**

1. Linear scan degrades with tool count: 10 tools = 10µs, 1000 tools = 1ms
2. No caching layer - same tool looked up repeatedly
3. Called on every invocation in agent loops (can be 10-50x per execution)
4. ListAll() may hit database on every call

**Fix - Add In-Memory Index:**

```go
// engine/mcp/registry/registry.go
type Registry struct {
    store     Store
    toolIndex sync.Map // map[string]*Tool - thread-safe
    mu        sync.RWMutex
    version   int64 // Incremented on changes
}

// O(1) lookup with cache
func (r *Registry) FindTool(ctx context.Context, toolName string) (*Tool, error) {
    // Fast path - check cache
    if tool, ok := r.toolIndex.Load(toolName); ok {
        return tool.(*Tool), nil
    }

    // Slow path - load from store and cache
    r.mu.Lock()
    defer r.mu.Unlock()

    // Double-check after acquiring lock
    if tool, ok := r.toolIndex.Load(toolName); ok {
        return tool.(*Tool), nil
    }

    tool, err := r.store.GetByName(ctx, toolName)
    if err != nil {
        return nil, err
    }

    r.toolIndex.Store(toolName, tool)
    return tool, nil
}

// Invalidate cache on mutations
func (r *Registry) Register(ctx context.Context, tool *Tool) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if err := r.store.Save(ctx, tool); err != nil {
        return err
    }

    r.toolIndex.Store(tool.Name, tool)
    r.version++
    return nil
}

func (r *Registry) Unregister(ctx context.Context, toolName string) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if err := r.store.Delete(ctx, toolName); err != nil {
        return err
    }

    r.toolIndex.Delete(toolName)
    r.version++
    return nil
}
```

**Alternative - Add Database Index:**

```sql
-- If store is SQL-based
CREATE INDEX CONCURRENTLY idx_mcp_tools_name
ON mcp_tools(name)
WHERE deleted_at IS NULL;

-- Add compound index for server + name lookups
CREATE INDEX CONCURRENTLY idx_mcp_tools_server_name
ON mcp_tools(server_id, name)
WHERE deleted_at IS NULL;
```

**Impact:**

- Reduces lookup from O(n) → O(1): 1ms → 10µs for 1000 tools
- Eliminates 90-95% of tool lookup latency
- Critical for agent loops invoking multiple tools

**Benchmarks:**

```go
// Add benchmark
func BenchmarkToolLookup(b *testing.B) {
    registry := setupRegistryWith1000Tools()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := registry.FindTool(ctx, "common-tool-500")
        if err != nil {
            b.Fatal(err)
        }
    }
}

// Before: BenchmarkToolLookup-8  2000  800000 ns/op
// After:  BenchmarkToolLookup-8  200000  8000 ns/op (100x faster)
```

**Testing:**

```bash
# Concurrency safety
go test -race -count=100 ./engine/mcp/registry

# Cache invalidation
go test -run TestRegistryMutationInvalidatesCache ./engine/mcp/registry

# Load test
go test -run TestToolLookupUnderLoad ./engine/mcp/registry
```

**Effort:** M (3-4h including tests)  
**Risk:** Medium - requires thread-safe cache invalidation

---

### 2. Missing Usage Metadata Persistence

**Location:** `engine/llm/provider/openai.go:234–289`, `engine/llm/provider/anthropic.go:198–245`

**Severity:** 🔴 HIGH

**Issue:**

```go
// Usage data is calculated but never persisted
func (p *Provider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    req.Model,
        Messages: convertMessages(req.Messages),
        // ...
    })

    if err != nil {
        return nil, err
    }

    // Usage calculated here
    usage := &UsageMetadata{
        PromptTokens:     resp.Usage.PromptTokens,
        CompletionTokens: resp.Usage.CompletionTokens,
        TotalTokens:      resp.Usage.TotalTokens,
    }

    // But usage is NOT saved to database
    // Lost when execution completes
    return &CompletionResponse{
        Content: resp.Choices[0].Message.Content,
        Usage:   usage, // Only returned, not persisted
    }, nil
}
```

**Problems:**

1. No historical usage tracking for cost analysis
2. Cannot audit token consumption per agent/task/workflow
3. Cannot detect usage anomalies or spikes
4. Billing reconciliation impossible

**Fix - Add Usage Repository:**

```go
// engine/llm/usage/repository.go (NEW)
package usage

import (
    "context"
    "time"
    "github.com/compozy/compozy/pkg/core"
)

type Record struct {
    ID               core.ID
    ExecutionID      core.ID
    Provider         string
    Model            string
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
    CostUSD          float64
    Timestamp        time.Time
    Metadata         map[string]any
}

type Repository interface {
    Save(ctx context.Context, record *Record) error
    GetByExecution(ctx context.Context, execID core.ID) ([]*Record, error)
    GetTotalUsage(ctx context.Context, start, end time.Time) (*AggregateSummary, error)
}

// engine/llm/provider/openai.go
func (p *Provider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    resp, err := p.client.CreateChatCompletion(ctx, req)
    if err != nil {
        return nil, err
    }

    usage := &UsageMetadata{
        PromptTokens:     resp.Usage.PromptTokens,
        CompletionTokens: resp.Usage.CompletionTokens,
        TotalTokens:      resp.Usage.TotalTokens,
    }

    // Persist usage asynchronously to avoid blocking
    go func() {
        saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        record := &usage.Record{
            ID:               core.NewID(),
            ExecutionID:      req.ExecutionID,
            Provider:         "openai",
            Model:            req.Model,
            PromptTokens:     usage.PromptTokens,
            CompletionTokens: usage.CompletionTokens,
            TotalTokens:      usage.TotalTokens,
            CostUSD:          calculateCost(req.Model, usage),
            Timestamp:        time.Now(),
        }

        if err := p.usageRepo.Save(saveCtx, record); err != nil {
            logger.FromContext(ctx).Error("failed to save usage", "error", err)
        }
    }()

    return &CompletionResponse{
        Content: resp.Choices[0].Message.Content,
        Usage:   usage,
    }, nil
}

// Cost calculation helper
func calculateCost(model string, usage *UsageMetadata) float64 {
    prices := map[string]struct{ input, output float64 }{
        "gpt-4-turbo":  {0.01 / 1000, 0.03 / 1000},
        "gpt-4":        {0.03 / 1000, 0.06 / 1000},
        "gpt-3.5-turbo": {0.0015 / 1000, 0.002 / 1000},
    }

    price, ok := prices[model]
    if !ok {
        return 0 // Unknown model
    }

    return float64(usage.PromptTokens)*price.input +
           float64(usage.CompletionTokens)*price.output
}
```

**Database Schema:**

```sql
CREATE TABLE llm_usage (
    id UUID PRIMARY KEY,
    execution_id UUID NOT NULL REFERENCES executions(id),
    provider VARCHAR(50) NOT NULL,
    model VARCHAR(100) NOT NULL,
    prompt_tokens INTEGER NOT NULL,
    completion_tokens INTEGER NOT NULL,
    total_tokens INTEGER NOT NULL,
    cost_usd DECIMAL(10, 6),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB,

    INDEX idx_usage_execution (execution_id),
    INDEX idx_usage_timestamp (timestamp DESC),
    INDEX idx_usage_provider_model (provider, model)
);
```

**Impact:**

- Enables cost tracking and billing
- Provides audit trail for compliance
- Allows usage analysis and optimization

**Effort:** M (4-5h including schema migration)  
**Risk:** Low - asynchronous write doesn't block critical path

---

### 3. Missing Provider Response Cache

**Location:** `engine/llm/provider/base.go:67–123`

**Severity:** 🟡 MEDIUM

**Issue:**

```go
// Same prompt sent multiple times = multiple API calls
func (p *BaseProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    // No cache check - always calls provider API
    return p.doComplete(ctx, req)
}
```

**Problem:** Identical requests (e.g., in retry loops) hit API unnecessarily

**Fix - Add Response Cache:**

```go
// engine/llm/cache/cache.go (NEW)
package cache

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "time"
)

type ResponseCache struct {
    store    Store // Redis or in-memory
    ttl      time.Duration
    disabled bool
}

func (c *ResponseCache) Get(ctx context.Context, req *CompletionRequest) (*CompletionResponse, bool) {
    if c.disabled {
        return nil, false
    }

    key := c.computeKey(req)
    return c.store.Get(ctx, key)
}

func (c *ResponseCache) Set(ctx context.Context, req *CompletionRequest, resp *CompletionResponse) error {
    if c.disabled {
        return nil
    }

    key := c.computeKey(req)
    return c.store.Set(ctx, key, resp, c.ttl)
}

func (c *ResponseCache) computeKey(req *CompletionRequest) string {
    // Hash deterministic request fields
    h := sha256.New()
    h.Write([]byte(req.Provider))
    h.Write([]byte(req.Model))
    for _, msg := range req.Messages {
        h.Write([]byte(msg.Role))
        h.Write([]byte(msg.Content))
    }
    return hex.EncodeToString(h.Sum(nil))
}

// In provider implementation:
func (p *Provider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    // Check cache first
    if cached, ok := p.cache.Get(ctx, req); ok {
        return cached, nil
    }

    resp, err := p.doComplete(ctx, req)
    if err != nil {
        return nil, err
    }

    // Cache successful responses
    _ = p.cache.Set(ctx, req, resp) // Ignore cache errors

    return resp, nil
}
```

**Configuration:**

```yaml
# config.yaml
llm:
  cache:
    enabled: true
    ttl: 1h
    max_size: 1000
```

**Impact:**

- Reduces API costs for repeated requests
- Faster response for cached content
- Opt-in per configuration

**Effort:** M (3h)  
**Risk:** Low - cache miss = no impact

---

## Medium Priority Issues

### 4. MCP Server Connection Pooling Missing

**Location:** `engine/mcp/client/client.go:89–145`

**Severity:** 🟡 MEDIUM

**Issue:**

```go
// New connection per request
func (c *Client) CallTool(ctx context.Context, serverID, toolName string, args map[string]any) (*Result, error) {
    conn, err := c.connect(ctx, serverID) // Fresh connection
    defer conn.Close()

    return conn.Invoke(ctx, toolName, args)
}
```

**Problem:** High connection overhead for frequently used MCP servers

**Fix - Add Connection Pool:**

```go
// engine/mcp/client/pool.go (NEW)
type ConnectionPool struct {
    pools map[string]*serverPool // serverID -> pool
    mu    sync.RWMutex
}

type serverPool struct {
    conns chan *Connection
    max   int
}

func (p *ConnectionPool) Get(ctx context.Context, serverID string) (*Connection, error) {
    p.mu.RLock()
    pool, exists := p.pools[serverID]
    p.mu.RUnlock()

    if !exists {
        return p.createConnection(ctx, serverID)
    }

    select {
    case conn := <-pool.conns:
        if conn.IsHealthy() {
            return conn, nil
        }
        conn.Close()
        return p.createConnection(ctx, serverID)
    default:
        return p.createConnection(ctx, serverID)
    }
}

func (p *ConnectionPool) Put(serverID string, conn *Connection) {
    p.mu.RLock()
    pool, exists := p.pools[serverID]
    p.mu.RUnlock()

    if !exists || !conn.IsHealthy() {
        conn.Close()
        return
    }

    select {
    case pool.conns <- conn:
    default:
        conn.Close() // Pool full
    }
}
```

**Impact:**

- Reduces connection latency from ~100ms to ~1ms
- Lower resource usage on MCP servers

**Effort:** L (5-6h)  
**Risk:** Medium - requires connection health checks

---

## Low Priority Issues

### 5. Inefficient Provider Factory Lookup

**Location:** `engine/llm/factory/factory.go:45–78`

**Issue:** Linear scan through provider list

**Fix:** Use map-based lookup

```go
type Factory struct {
    providers map[string]ProviderConstructor
}

func (f *Factory) Create(providerName string) (Provider, error) {
    constructor, ok := f.providers[providerName]
    if !ok {
        return nil, ErrUnknownProvider
    }
    return constructor(), nil
}
```

**Effort:** S (1h)

---

## Implementation Plan

### Phase 1 - Critical (Week 1)

- [ ] Add O(1) tool lookup index (3h)
- [ ] Implement usage persistence (5h)
- [ ] Add database indexes and migration

### Phase 2 - Caching (Week 2)

- [ ] Implement response cache (3h)
- [ ] Add cache metrics (1h)
- [ ] Configure cache TTL policies

### Phase 3 - Optimizations (Week 3)

- [ ] Add MCP connection pooling (6h)
- [ ] Optimize provider factory (1h)
- [ ] Load testing and tuning

---

## Testing Requirements

```bash
# Tool lookup performance
go test -bench=BenchmarkToolLookup -benchmem ./engine/mcp/registry

# Usage persistence
go test -run TestUsagePersistence ./engine/llm/usage

# Cache effectiveness
go test -run TestResponseCache ./engine/llm/cache

# Connection pool
go test -race -run TestConnectionPool ./engine/mcp/client

# Integration
make test
```

---

## Success Metrics

| Metric                           | Before | Target | Measurement |
| -------------------------------- | ------ | ------ | ----------- |
| Tool lookup latency (1000 tools) | 1ms    | 10µs   | Benchmark   |
| Usage data loss rate             | 100%   | 0%     | Monitor DB  |
| Cache hit rate                   | N/A    | >30%   | Metrics     |
| MCP connection overhead          | 100ms  | 1-5ms  | Tracing     |
| Provider lookup time             | 10µs   | 1µs    | Benchmark   |

---

**Document Version:** 1.0  
**Last Updated:** 2025-01-16  
**Owner:** Platform Team
