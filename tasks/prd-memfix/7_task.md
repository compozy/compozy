---
status: completed # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/memory,engine/infra</domain>
<type>implementation</type>
<scope>performance</scope>
<complexity>medium</complexity>
<dependencies>task_1.0,task_2.0,task_3.0</dependencies>
</task_context>

# Task 7.0: Critical Performance Optimizations

## Overview

Implement two critical performance optimizations identified during analysis: move token counting to async background processing to eliminate 10-50ms latency per append operation, and make Redis connection pool configurable with production-ready defaults to prevent bottlenecks at scale.

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** check dependent files APIs before write tests to avoid write wrong code
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing ANY subtask
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

## Subtasks

- [ ] 7.1 Implement async token counter interface and background processor
- [ ] 7.2 Modify memory append operation to use async token counting
- [ ] 7.3 Add environment variable support for Redis pool configuration
- [ ] 7.4 Update default Redis pool size to production-ready value
- [ ] 7.5 Create tests for async token counting
- [ ] 7.6 Add performance metrics for monitoring improvements

## Implementation Details

### Async Token Counter Implementation

**File: `engine/memory/tokens/async_counter.go`**

```go
package tokens

import (
    "context"
    "sync"
    "time"

    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/pkg/logger"
)

// AsyncTokenCounter processes token counting in the background
type AsyncTokenCounter struct {
    realCounter TokenCounter
    queue       chan *tokenCountRequest
    workers     int
    wg          sync.WaitGroup
    log         logger.Logger
    metrics     *TokenMetrics
}

type tokenCountRequest struct {
    ctx         context.Context
    memoryRef   core.MemoryReference
    message     *core.Message
    resultChan  chan<- tokenCountResult
}

type tokenCountResult struct {
    count int
    err   error
}

// NewAsyncTokenCounter creates a new async token counter
func NewAsyncTokenCounter(counter TokenCounter, workers int, log logger.Logger) *AsyncTokenCounter {
    if workers <= 0 {
        workers = 10 // Default worker pool size
    }

    atc := &AsyncTokenCounter{
        realCounter: counter,
        queue:       make(chan *tokenCountRequest, 1000), // Buffered queue
        workers:     workers,
        log:         log,
        metrics:     NewTokenMetrics(),
    }

    atc.start()
    return atc
}

// start initializes the worker pool
func (atc *AsyncTokenCounter) start() {
    for i := 0; i < atc.workers; i++ {
        atc.wg.Add(1)
        go atc.worker(i)
    }
}

// worker processes token count requests
func (atc *AsyncTokenCounter) worker(id int) {
    defer atc.wg.Done()

    for req := range atc.queue {
        start := time.Now()

        count, err := atc.realCounter.CountTokens(req.ctx, req.message)

        atc.metrics.RecordDuration(time.Since(start))

        if req.resultChan != nil {
            req.resultChan <- tokenCountResult{
                count: count,
                err:   err,
            }
        }

        if err != nil {
            atc.log.Error("Failed to count tokens",
                "error", err,
                "memory_ref", req.memoryRef.ID,
                "worker_id", id,
            )
            atc.metrics.IncrementErrors()
        } else {
            atc.metrics.IncrementSuccess()
        }
    }
}

// ProcessAsync queues a message for token counting without blocking
func (atc *AsyncTokenCounter) ProcessAsync(ctx context.Context, memoryRef core.MemoryReference, message *core.Message) {
    select {
    case atc.queue <- &tokenCountRequest{
        ctx:       ctx,
        memoryRef: memoryRef,
        message:   message,
    }:
        // Successfully queued
    default:
        // Queue full, log and continue
        atc.log.Warn("Token counter queue full, skipping message",
            "memory_ref", memoryRef.ID,
        )
        atc.metrics.IncrementDropped()
    }
}

// ProcessWithResult queues a message and waits for the result
func (atc *AsyncTokenCounter) ProcessWithResult(ctx context.Context, memoryRef core.MemoryReference, message *core.Message) (int, error) {
    resultChan := make(chan tokenCountResult, 1)

    select {
    case atc.queue <- &tokenCountRequest{
        ctx:        ctx,
        memoryRef:  memoryRef,
        message:    message,
        resultChan: resultChan,
    }:
        // Wait for result with timeout
        select {
        case result := <-resultChan:
            return result.count, result.err
        case <-ctx.Done():
            return 0, ctx.Err()
        case <-time.After(5 * time.Second):
            return 0, fmt.Errorf("token counting timeout")
        }
    default:
        return 0, fmt.Errorf("token counter queue full")
    }
}

// Shutdown gracefully stops the async counter
func (atc *AsyncTokenCounter) Shutdown() {
    close(atc.queue)
    atc.wg.Wait()
}
```

### Memory Instance Modification

**File: `engine/memory/instance/memory_instance.go`** (modify Append method)

```go
// Around line 150, replace synchronous token counting with:

// Queue async token counting (non-blocking)
if mi.asyncTokenCounter != nil {
    mi.asyncTokenCounter.ProcessAsync(ctx, mi.ref, msg)
} else {
    // Fallback to synchronous if async not available
    tokenCount := mi.calculateMessageTokenCount(ctx, msg)
    msg.Metadata["token_count"] = tokenCount
}

// Continue with append without waiting for token count
```

### Redis Pool Configuration

**File: `engine/infra/cache/redis.go`** (modify NewRedis function)

```go
import (
    "os"
    "strconv"
)

// Around line 303-305, replace with:

// Configure pool size from environment with production default
if cfg.PoolSize == 0 {
    poolSizeStr := os.Getenv("REDIS_POOL_SIZE")
    if poolSizeStr != "" {
        if poolSize, err := strconv.Atoi(poolSizeStr); err == nil && poolSize > 0 {
            cfg.PoolSize = poolSize
        } else {
            log.Warn("Invalid REDIS_POOL_SIZE value, using default",
                "value", poolSizeStr,
                "default", 100,
            )
            cfg.PoolSize = 100
        }
    } else {
        cfg.PoolSize = 100 // Production-ready default
    }
}

// Also make other pool settings configurable
if cfg.PoolTimeout == 0 {
    timeoutStr := os.Getenv("REDIS_POOL_TIMEOUT")
    if timeoutStr != "" {
        if timeout, err := time.ParseDuration(timeoutStr); err == nil {
            cfg.PoolTimeout = timeout
        } else {
            cfg.PoolTimeout = 30 * time.Second
        }
    } else {
        cfg.PoolTimeout = 30 * time.Second
    }
}

if cfg.MaxIdleConns == 0 {
    maxIdleStr := os.Getenv("REDIS_MAX_IDLE_CONNS")
    if maxIdleStr != "" {
        if maxIdle, err := strconv.Atoi(maxIdleStr); err == nil && maxIdle > 0 {
            cfg.MaxIdleConns = maxIdle
        } else {
            cfg.MaxIdleConns = 50
        }
    } else {
        cfg.MaxIdleConns = 50
    }
}
```

### Token Metrics Implementation

**File: `engine/memory/tokens/metrics.go`**

```go
package tokens

import (
    "sync/atomic"
    "time"
)

// TokenMetrics tracks token counting performance
type TokenMetrics struct {
    successCount  atomic.Uint64
    errorCount    atomic.Uint64
    droppedCount  atomic.Uint64
    totalDuration atomic.Uint64
    countDuration atomic.Uint64
}

func NewTokenMetrics() *TokenMetrics {
    return &TokenMetrics{}
}

func (tm *TokenMetrics) IncrementSuccess() {
    tm.successCount.Add(1)
}

func (tm *TokenMetrics) IncrementErrors() {
    tm.errorCount.Add(1)
}

func (tm *TokenMetrics) IncrementDropped() {
    tm.droppedCount.Add(1)
}

func (tm *TokenMetrics) RecordDuration(d time.Duration) {
    tm.totalDuration.Add(uint64(d.Nanoseconds()))
    tm.countDuration.Add(1)
}

func (tm *TokenMetrics) GetStats() map[string]interface{} {
    count := tm.countDuration.Load()
    avgDuration := time.Duration(0)
    if count > 0 {
        avgDuration = time.Duration(tm.totalDuration.Load() / count)
    }

    return map[string]interface{}{
        "success_count":   tm.successCount.Load(),
        "error_count":     tm.errorCount.Load(),
        "dropped_count":   tm.droppedCount.Load(),
        "avg_duration_ns": avgDuration.Nanoseconds(),
        "avg_duration_ms": avgDuration.Milliseconds(),
    }
}
```

### Tests for Async Token Counter

**File: `engine/memory/tokens/async_counter_test.go`**

```go
package tokens

import (
    "context"
    "testing"
    "time"

    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

type mockTokenCounter struct {
    mock.Mock
}

func (m *mockTokenCounter) CountTokens(ctx context.Context, msg *core.Message) (int, error) {
    args := m.Called(ctx, msg)
    return args.Int(0), args.Error(1)
}

func TestAsyncTokenCounter_ProcessAsync(t *testing.T) {
    t.Run("Should process token counting asynchronously", func(t *testing.T) {
        // Arrange
        mockCounter := new(mockTokenCounter)
        log := logger.NewTest()
        asyncCounter := NewAsyncTokenCounter(mockCounter, 2, log)
        defer asyncCounter.Shutdown()

        msg := &core.Message{
            Role:    "user",
            Content: "Test message",
        }
        ref := core.MemoryReference{ID: "test_memory"}

        mockCounter.On("CountTokens", mock.Anything, msg).
            Return(10, nil).
            After(50 * time.Millisecond) // Simulate processing time

        // Act
        start := time.Now()
        asyncCounter.ProcessAsync(context.Background(), ref, msg)
        duration := time.Since(start)

        // Assert
        assert.Less(t, duration, 10*time.Millisecond,
            "ProcessAsync should return immediately")

        // Wait for async processing
        time.Sleep(100 * time.Millisecond)
        mockCounter.AssertExpectations(t)
    })

    t.Run("Should handle queue full gracefully", func(t *testing.T) {
        // Arrange
        mockCounter := new(mockTokenCounter)
        log := logger.NewTest()
        asyncCounter := NewAsyncTokenCounter(mockCounter, 1, log)
        asyncCounter.queue = make(chan *tokenCountRequest, 1) // Small queue
        defer asyncCounter.Shutdown()

        msg := &core.Message{Content: "Test"}
        ref := core.MemoryReference{ID: "test"}

        // Fill the queue
        asyncCounter.queue <- &tokenCountRequest{}

        // Act - should not block
        asyncCounter.ProcessAsync(context.Background(), ref, msg)

        // Assert
        stats := asyncCounter.metrics.GetStats()
        assert.Equal(t, uint64(1), stats["dropped_count"])
    })
}

func TestAsyncTokenCounter_ProcessWithResult(t *testing.T) {
    t.Run("Should return token count result", func(t *testing.T) {
        // Arrange
        mockCounter := new(mockTokenCounter)
        log := logger.NewTest()
        asyncCounter := NewAsyncTokenCounter(mockCounter, 2, log)
        defer asyncCounter.Shutdown()

        msg := &core.Message{Content: "Test message"}
        ref := core.MemoryReference{ID: "test"}

        mockCounter.On("CountTokens", mock.Anything, msg).Return(15, nil)

        // Act
        count, err := asyncCounter.ProcessWithResult(
            context.Background(), ref, msg)

        // Assert
        assert.NoError(t, err)
        assert.Equal(t, 15, count)
        mockCounter.AssertExpectations(t)
    })
}
```

### Environment Configuration Documentation

**File: `docs/configuration/redis-performance.md`**

````markdown
# Redis Performance Configuration

The memory system now supports environment-based configuration for Redis connection pooling to optimize performance at scale.

## Environment Variables

### REDIS_POOL_SIZE

Controls the maximum number of connections in the Redis connection pool.

- **Default**: 100 (production-ready)
- **Development**: 10-20 connections
- **Production**: 100-500 depending on load
- **Example**: `REDIS_POOL_SIZE=200`

### REDIS_POOL_TIMEOUT

Maximum time to wait for a connection from the pool.

- **Default**: 30s
- **Format**: Go duration string (e.g., "30s", "1m", "500ms")
- **Example**: `REDIS_POOL_TIMEOUT=45s`

### REDIS_MAX_IDLE_CONNS

Number of idle connections to maintain in the pool.

- **Default**: 50
- **Recommendation**: Set to 50% of REDIS_POOL_SIZE
- **Example**: `REDIS_MAX_IDLE_CONNS=100`

## Performance Tuning Guide

### For High Throughput (>1000 ops/sec)

```bash
export REDIS_POOL_SIZE=300
export REDIS_MAX_IDLE_CONNS=150
export REDIS_POOL_TIMEOUT=45s
```
````

### For Burst Traffic

```bash
export REDIS_POOL_SIZE=500
export REDIS_MAX_IDLE_CONNS=100
export REDIS_POOL_TIMEOUT=60s
```

### Monitoring

Monitor these metrics to tune pool size:

- Connection wait time
- Pool exhaustion events
- Active vs idle connections
- Redis operation latency

```

### Relevant Files

> Files that this task will create/modify:
- `engine/memory/tokens/async_counter.go` - New async token counter
- `engine/memory/tokens/metrics.go` - Token counting metrics
- `engine/memory/tokens/async_counter_test.go` - Tests for async counter
- `engine/memory/instance/memory_instance.go` - Modified append method
- `engine/infra/cache/redis.go` - Redis pool configuration
- `docs/configuration/redis-performance.md` - Configuration documentation

### Dependent Files

> Files that must be checked for compatibility:
- `engine/memory/manager.go` - Needs to initialize async counter
- `engine/memory/tokens/unified_counter.go` - Existing token counter
- `engine/memory/instance/operations.go` - Memory operations
- `engine/worker/mod.go` - Worker initialization

## Success Criteria
- [ ] Token counting no longer blocks append operations
- [ ] Redis pool size configurable via environment variable
- [ ] Default pool size increased from 10 to 100
- [ ] Async token counter handles queue overflow gracefully
- [ ] Performance metrics show <1ms append latency (excluding Redis)
- [ ] No regression in token counting accuracy
- [ ] Tests verify async behavior and error handling
- [ ] Documentation clearly explains configuration options
```
