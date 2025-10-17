---
title: "Inefficient Provider Factory Lookup"
group: "ENGINE_GROUP_2_LLM_TOOLS"
category: "performance"
priority: ""
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_2_LLM_TOOLS_PERFORMANCE.md"
issue_index: "5"
sequence: "11"
---

## Inefficient Provider Factory Lookup

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

## Success Metrics

| Metric                           | Before | Target | Measurement |
| -------------------------------- | ------ | ------ | ----------- |
| Tool lookup latency (1000 tools) | 1ms    | 10µs   | Benchmark   |
| Usage data loss rate             | 100%   | 0%     | Monitor DB  |
| Cache hit rate                   | N/A    | >30%   | Metrics     |
| MCP connection overhead          | 100ms  | 1-5ms  | Tracing     |
| Provider lookup time             | 10µs   | 1µs    | Benchmark   |

**Document Version:** 1.0  
**Last Updated:** 2025-01-16  
**Owner:** Platform Team
