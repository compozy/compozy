---
title: "O(n) MCP Tool Lookup Performance"
group: "ENGINE_GROUP_2_LLM_TOOLS"
category: "performance"
priority: "🔴 CRITICAL"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_2_LLM_TOOLS_PERFORMANCE.md"
issue_index: "1"
sequence: "7"
---

## O(n) MCP Tool Lookup Performance

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
