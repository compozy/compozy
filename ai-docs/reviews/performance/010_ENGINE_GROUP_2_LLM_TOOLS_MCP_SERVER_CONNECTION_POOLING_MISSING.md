---
title: "MCP Server Connection Pooling Missing"
group: "ENGINE_GROUP_2_LLM_TOOLS"
category: "performance"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_2_LLM_TOOLS_PERFORMANCE.md"
issue_index: "4"
sequence: "10"
---

## MCP Server Connection Pooling Missing

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

## Low Priority Issues
