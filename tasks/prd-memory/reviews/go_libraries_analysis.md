# Go Libraries Analysis for Engine/Memory Package Optimization

## Executive Summary

This analysis identifies Go libraries that could replace custom implementations in the engine/memory package, potentially reducing maintenance burden while maintaining or improving functionality. Based on deep research using Perplexity AI, we've identified several production-ready libraries that align with Compozy's architecture and requirements.

## Current Implementation Overview

The engine/memory package currently implements:

### 1. Token Management

- **Current**: Custom tiktoken-go integration with manual token counting
- **Features**: Max token limits, context ratio management, token metadata tracking

### 2. Redis Operations

- **Current**: Custom Lua scripts for atomic operations
- **Features**: TTL management, metadata tracking, atomic append operations

### 3. Distributed Locking

- **Current**: Reuses engine/infra/cache LockManager
- **Features**: Key-based locking, timeout handling

### 4. Privacy Management

- **Current**: Custom regex-based redaction with circuit breakers
- **Features**: Pattern matching, selective persistence, failure protection

### 5. Memory Storage

- **Current**: Custom Redis abstraction layer
- **Features**: Append-only design, JSON serialization, TTL support

### 6. Template Processing

- **Current**: Reuses pkg/tplengine
- **Features**: Dynamic key generation, variable interpolation

## Library Recommendations by Component

### 1. Token Management Enhancement

#### Replace Custom Token Counting

**Current Implementation**: Direct tiktoken-go usage limited to OpenAI models

**Recommended Library**: `github.com/open-and-sustainable/alembica/llm/tokens`

- **Benefits**:
    - Unified interface for OpenAI, GoogleAI, Cohere, Anthropic, DeepSeek
    - Eliminates need for provider-specific implementations
    - Real-time token estimation via provider APIs
- **Migration Effort**: Low
- **Code Reduction**: ~200 lines (multiple provider implementations)

```go
// Before (custom implementation per provider)
func countTokensOpenAI(text string) int { /* custom */ }
func countTokensAnthropic(text string) int { /* custom */ }

// After
counter := tokens.RealTokenCounter{}
numTokens := counter.GetNumTokensFromPrompt(text, provider, model, apiKey)
```

#### Add Token-Based Eviction

**Current Implementation**: Custom FIFO strategy

**Recommended Library**: `github.com/golanguzb70/lrucache` with token cost functions

- **Benefits**:
    - Generic LRU with custom cost calculation
    - Time-based eviction support
    - Thread-safe operations
- **Migration Effort**: Medium
- **Code Reduction**: ~150 lines (eviction logic)

### 2. Redis Operations Simplification

#### Replace Lua Scripts

**Current Implementation**: Complex Lua scripts for atomic operations

**Recommended Library**: `github.com/redis/go-redis` pipelines + `github.com/hibiken/asynq`

- **Benefits**:
    - Pipeline transactions replace most Lua scripts
    - Asynq provides built-in persistence patterns
    - Better error handling and retries
- **Migration Effort**: Medium
- **Code Reduction**: ~300 lines (Lua scripts + error handling)

```go
// Before (Lua script)
luaAppend := redis.NewScript(`...complex lua...`)

// After (pipeline)
pipe := client.Pipeline()
pipe.RPush(ctx, key, data)
pipe.Expire(ctx, key, ttl)
pipe.HSet(ctx, metaKey, metadata)
_, err := pipe.Exec(ctx)
```

### 3. Circuit Breaker Enhancement

#### Replace Custom Implementation

**Current Implementation**: Basic circuit breaker in privacy manager

**Recommended Library**: `github.com/slok/goresilience`

- **Benefits**:
    - Modular resilience patterns (retry, timeout, bulkhead)
    - Prometheus metrics integration
    - Middleware chaining for complex scenarios
- **Migration Effort**: Low
- **Code Reduction**: ~100 lines

```go
// After
runner := runnerchain.New(
    timeout.NewMiddleware(timeout.Config{Timeout: 100*time.Millisecond}),
    circuitbreaker.NewMiddleware(circuitbreaker.Config{
        ErrorPercentThresholdToOpen:        50,
        MinimumRequestToOpen:               10,
        SuccessfulRequiredOnHalfOpen:       1,
        WaitDurationInOpenState:            5 * time.Second,
        MetricsSlidingWindowBuckets:        10,
        MetricsBucketDuration:              1 * time.Second,
    }),
    retry.NewMiddleware(retry.Config{
        Times: 3,
    }),
)
```

### 4. Distributed Locking (Already Optimized)

**Current Implementation**: Reuses engine/infra/cache LockManager
**Assessment**: Already using best practices, no replacement needed

### 5. Cache Management for Memory Instances

#### Add Priority-Based Eviction

**Current Implementation**: No caching of memory instances

**Recommended Library**: `github.com/hashicorp/golang-lru` with custom eviction

- **Benefits**:
    - ARC algorithm for better hit rates
    - Built-in metrics
    - Thread-safe implementations
- **Migration Effort**: Medium
- **New Capability**: Instance pooling and reuse

### 6. Template Processing (Already Optimized)

**Current Implementation**: Reuses pkg/tplengine
**Assessment**: Already optimized, no replacement needed

### 7. Async Operations Enhancement

#### Complement Temporal with Lightweight Tasks

**Current Implementation**: All async via Temporal activities

**Recommended Library**: `github.com/hibiken/asynq` for lightweight operations

- **Benefits**:
    - Redis-based task queue for simple operations
    - Scheduled jobs without Temporal overhead
    - Built-in monitoring UI
- **Migration Effort**: Low
- **Use Case**: Background cleanup, metrics collection

## Implementation Priority Matrix

| Component             | Library              | Effort | Impact | Priority |
| --------------------- | -------------------- | ------ | ------ | -------- |
| Multi-Provider Tokens | alembica/llm/tokens  | Low    | High   | **P0**   |
| Circuit Breaker       | goresilience         | Low    | Medium | **P0**   |
| Redis Lua Replacement | go-redis pipelines   | Medium | High   | **P1**   |
| Token-Based Eviction  | golanguzb70/lrucache | Medium | Medium | **P1**   |
| Lightweight Tasks     | asynq                | Low    | Low    | **P2**   |
| Instance Caching      | golang-lru           | Medium | Low    | **P2**   |

## Migration Strategy

### Phase 1: High-Impact, Low-Effort (2 weeks)

1. **Multi-Provider Token Support**

    - Integrate alembica/llm/tokens
    - Create provider registry
    - Update MemoryConfig to support provider selection

2. **Circuit Breaker Enhancement**
    - Replace custom implementation with goresilience
    - Add retry and timeout middleware
    - Integrate Prometheus metrics

### Phase 2: Core Optimization (4 weeks)

3. **Redis Operations**

    - Convert Lua scripts to go-redis pipelines
    - Implement transaction patterns
    - Add connection pooling optimizations

4. **Eviction Strategy**
    - Implement token-cost-aware LRU
    - Add priority-based eviction
    - Create eviction metrics

### Phase 3: Advanced Features (2 weeks)

5. **Background Tasks**
    - Set up asynq for non-critical operations
    - Move cleanup tasks from Temporal
    - Add scheduled maintenance jobs

## Code Reduction Analysis

### Total Estimated Reduction

- **Token Management**: ~200 lines
- **Redis Operations**: ~300 lines
- **Circuit Breaker**: ~100 lines
- **Eviction Logic**: ~150 lines
- **Total**: ~750 lines (25% of package)

### New Capabilities Gained

- Multi-provider token support
- Advanced resilience patterns
- Built-in metrics and monitoring
- Scheduled background tasks
- Instance pooling

## Risk Assessment

### Low Risk

- Token counting libraries (well-tested APIs)
- Circuit breaker (drop-in replacement)
- LRU cache (standard algorithms)

### Medium Risk

- Redis pipeline conversion (requires testing atomicity)
- Eviction strategy changes (may affect memory behavior)

### Mitigation Strategies

1. Feature flags for gradual rollout
2. Comprehensive integration tests
3. Performance benchmarks before/after
4. Rollback procedures for each phase

## Conclusion

The identified libraries offer significant opportunities to reduce custom code while adding advanced capabilities. The phased approach minimizes risk while delivering quick wins through multi-provider token support and enhanced resilience patterns.

Key benefits:

- 25% code reduction
- Multi-provider LLM support
- Production-grade resilience
- Better monitoring and metrics
- Reduced maintenance burden

The migration aligns with Compozy's architecture principles of reusing proven components while maintaining flexibility for domain-specific requirements.
