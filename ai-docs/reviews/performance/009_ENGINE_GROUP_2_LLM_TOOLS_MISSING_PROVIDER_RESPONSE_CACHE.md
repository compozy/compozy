---
title: "Missing Provider Response Cache"
group: "ENGINE_GROUP_2_LLM_TOOLS"
category: "performance"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_2_LLM_TOOLS_PERFORMANCE.md"
issue_index: "3"
sequence: "9"
---

## Missing Provider Response Cache

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

## Medium Priority Issues
