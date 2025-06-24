# Consolidated Memory Engine Implementation Plan

## Executive Summary

This document consolidates the implementation gap analysis with library recommendations to create an optimized implementation plan that:

- Addresses all identified gaps and TODOs
- Leverages existing Go libraries to avoid reinventing the wheel
- Reduces code complexity and maintenance burden
- Follows Compozy's architectural principles

## Implementation Overview

### Key Principles

1. **Use existing infrastructure first** (ConfigRegistry, Template Engine, LockManager)
2. **Adopt proven libraries** for complex functionality (token counting, resilience)
3. **Avoid overengineering** - simple solutions for simple problems
4. **Maintain consistency** with Compozy patterns

### Total Effort Reduction

- Original estimate: 4 weeks for gap implementation
- With libraries: 3.5 weeks (12% reduction after consensus analysis)
- Code reduction: ~750 lines (25% of package)
- **Updated after consensus review**: Distributed systems complexity requires additional time

## Phase 1: Critical Infrastructure Fixes (Week 1)

### Task 15.0: Configuration Loading with Registry

**Complexity**: LOW (2/10) → **TRIVIAL (1/10)** with existing infrastructure  
**Time**: 2 days → 0.5 days

#### Implementation

```go
// No new code needed - just wire existing components
func (mm *Manager) loadMemoryConfig(resourceID string) (*memcore.Resource, error) {
    config, err := mm.resourceRegistry.Get("memory", resourceID)
    if err != nil {
        return nil, memcore.NewConfigError(
            fmt.Sprintf("memory resource '%s' not found", resourceID),
            err,
        )
    }
    memResource, ok := config.(*memcore.Resource)
    if !ok {
        return nil, memcore.NewConfigError(
            fmt.Sprintf("invalid config type for '%s'", resourceID),
            fmt.Errorf("expected *memcore.Resource, got %T", config),
        )
    }
    return memResource, nil
}
```

**Libraries**: None - uses existing `autoload.ConfigRegistry`

### Task 16.0: Template Engine Integration

**Complexity**: LOW (2/10) → **TRIVIAL (1/10)** with existing engine  
**Time**: 2 days → 0.5 days

#### Implementation

```go
// Direct integration with existing template engine
func (mm *Manager) resolveMemoryKey(
    ctx context.Context,
    keyTemplate string,
    workflowContextData map[string]any,
) (string, string) {
    result, err := mm.tplEngine.ProcessString(keyTemplate, workflowContextData)
    if err != nil {
        mm.log.Warn("Failed to evaluate key template",
            "template", keyTemplate, "error", err)
        sanitizedKey := mm.sanitizeKey(keyTemplate)
        projectIDVal := extractProjectID(workflowContextData)
        return sanitizedKey, projectIDVal
    }
    return mm.sanitizeKey(result.Text), extractProjectID(workflowContextData)
}
```

**Libraries**: None - uses existing `pkg/tplengine`

### Task 17.0: Distributed Lock Manager

**Complexity**: MEDIUM (5/10) → **HIGH (7/10)** after consensus analysis  
**Time**: 3 days → 2 days (updated after expert review)
**⚠️ COMPLEXITY WARNING**: Distributed locking requires extensive testing for race conditions, deadlocks, and network partition scenarios

#### Implementation

```go
// Reuse existing LockManager with proper implementation
type lockManagerAdapter struct {
    lockManager *cache.LockManager
}

func (lma *lockManagerAdapter) Lock(ctx context.Context, key string, ttl time.Duration) (instance.Lock, error) {
    return lma.lockManager.AcquireLock(ctx, key, ttl)
}
```

**Libraries**: None - uses existing `engine/infra/cache/lock_manager.go`

### Task 18.0: Error Logging

**Complexity**: LOW (1/10) → **TRIVIAL (0.5/10)**  
**Time**: 0.5 days → 2 hours

Simple addition of error logging to existing code.

### NEW Task 18.5: Multi-Provider Token Counting

**Complexity**: NEW - LOW (3/10)** with library  
**Time\*\*: 1 day

#### Implementation

```go
import "github.com/open-and-sustainable/alembica/llm/tokens"

type UnifiedTokenCounter struct {
    realCounter *tokens.RealTokenCounter
    fallback    memcore.TokenCounter // existing tiktoken
}

func (u *UnifiedTokenCounter) CountTokens(ctx context.Context, text string) (int, error) {
    // Try real-time API counting first
    if count, err := u.realCounter.GetNumTokensFromPrompt(text, provider, model, apiKey); err == nil {
        return count, nil
    }
    // Fallback to tiktoken
    return u.fallback.CountTokens(ctx, text)
}
```

**Libraries**: `github.com/open-and-sustainable/alembica/llm/tokens`

- Supports OpenAI, GoogleAI, Cohere, Anthropic, DeepSeek
- Real-time token counting via provider APIs
- Eliminates provider-specific implementations

### Task 21.0: Enhanced Circuit Breaker (moved from Phase 2)

**Complexity**: NEW - HIGH (7/10)** after consensus analysis (moved to Phase 1)  
**Time**: 2 days (requires performance testing and tuning)
**⚠️ COMPLEXITY WARNING\*\*: Tuning resilience patterns requires extensive performance testing under various load conditions

#### Implementation

```go
import "github.com/slok/goresilience"

// Replace custom circuit breaker in privacy manager
runner := runnerchain.New(
    timeout.NewMiddleware(timeout.Config{Timeout: 100*time.Millisecond}),
    circuitbreaker.NewMiddleware(circuitbreaker.Config{
        ErrorPercentThresholdToOpen: 50,
        MinimumRequestToOpen: 10,
        WaitDurationInOpenState: 5 * time.Second,
    }),
    retry.NewMiddleware(retry.Config{Times: 3}),
)
```

**Libraries**: `github.com/slok/goresilience`

- Modular resilience patterns
- Prometheus metrics integration
- Replaces ~100 lines of custom code

## Phase 2: Core Features with Libraries (Week 2)

### Task 19.0: Flush Strategies with Library Support

**Complexity**: HIGH (7/10) → **MEDIUM (4/10)** with libraries  
**Time**: 5 days → 3 days

#### Updated Implementation Plan

**19.1: LRU Strategy** - Use library instead of custom

```go
import "github.com/hashicorp/golang-lru"

type LRUFlushStrategy struct {
    cache *lru.ARCCache // Adaptive Replacement Cache
}

func NewLRUStrategy(size int) (*LRUFlushStrategy, error) {
    cache, err := lru.NewARC(size)
    if err != nil {
        return nil, err
    }
    return &LRUFlushStrategy{cache: cache}, nil
}
```

**19.2: Token-Cost-Aware LRU** - Use specialized library

```go
import "github.com/golanguzb70/lrucache"

type TokenAwareLRU struct {
    cache *lrucache.LRUCache
}

func NewTokenAwareLRU(maxTokens int) *TokenAwareLRU {
    cache := lrucache.NewLRUCache(
        lrucache.WithCapacity(maxTokens),
        lrucache.WithCostFunc(func(key, value interface{}) int64 {
            msg := value.(memcore.MessageWithTokens)
            return int64(msg.TokenCount)
        }),
    )
    return &TokenAwareLRU{cache: cache}
}
```

**Libraries**:

- `github.com/hashicorp/golang-lru` - ARC algorithm, built-in metrics
- `github.com/golanguzb70/lrucache` - Custom cost functions for tokens

### Task 20.0: Eviction Policies

**Complexity**: MEDIUM (5/10) → **LOW (2/10)** with library patterns  
**Time**: 3 days → 1 day

Leverage the same LRU libraries for eviction policies.

### Task 22.0: Token Allocation System

**Complexity**: MEDIUM (6/10) → **LOW (3/10)** with simplified approach  
**Time**: 3 days → 1.5 days

Use library-provided cost functions and priorities.

### NEW Task 22.5: Redis Operations Optimization

**Complexity**: NEW - MEDIUM (4/10)\*\*  
**Time**: 2 days

#### Implementation

```go
// Replace complex Lua scripts with pipelines
func (s *RedisMemoryStore) AppendMessageWithTokenCount(
    ctx context.Context,
    key string,
    msg llm.Message,
    tokenCount int,
) error {
    pipe := s.client.Pipeline()

    // Atomic operations via pipeline
    msgBytes, _ := json.Marshal(msg)
    pipe.RPush(ctx, s.fullKey(key), msgBytes)
    pipe.HIncrBy(ctx, s.metadataKey(key), "message_count", 1)
    pipe.HIncrBy(ctx, s.metadataKey(key), "token_count", int64(tokenCount))

    _, err := pipe.Exec(ctx)
    return err
}
```

**Benefits**:

- Removes ~300 lines of Lua scripts
- Better error handling
- Easier to maintain

## Phase 3: Testing & Advanced Features (Week 3)

### Testing Tasks (25-28)

**Time Reduction**: 4 days → 2.5 days

- Simplified by using well-tested libraries
- Less custom code to test

### Task 21.0: AI-Based Summarizer

**Complexity**: HIGH (8/10) → **MEDIUM (5/10)**  
**Time**: 3 days → 2 days

Can leverage existing LLM integration more effectively.

### Task 24.0: Metrics Completion

**Complexity**: MEDIUM (4/10) → **LOW (2/10)** with goresilience  
**Time**: 2 days → 1 day

Goresilience provides built-in Prometheus metrics.

### NEW Task 29.0: Lightweight Background Tasks

**Complexity**: NEW - LOW (3/10)\*\*  
**Time**: 1 day

#### Implementation

```go
import "github.com/hibiken/asynq"

// For non-critical background operations
server := asynq.NewServer(
    asynq.RedisClientOpt{Addr: redisAddr},
    asynq.Config{Concurrency: 10},
)

// Schedule cleanup tasks
scheduler := asynq.NewScheduler(
    asynq.RedisClientOpt{Addr: redisAddr},
    &asynq.SchedulerOpts{},
)

// Use for: metrics collection, cache warming, non-critical cleanup
```

**Libraries**: `github.com/hibiken/asynq`

- Redis-based task queue
- Complements Temporal for lightweight operations
- Built-in monitoring UI

## Implementation Timeline

### Week 1: Critical Infrastructure (5 days)

- Day 1: Tasks 15-16 (Config & Template) - 1 day total
- Day 2-3: Task 17 (Distributed Locking) - 2 days (updated after consensus)
- Day 3.5: Task 18 (Error Logging) - 0.5 days
- Day 4: Task 18.5 (Multi-Provider Tokens) - 1 day
- Day 5: Task 22.5 (Redis Optimization) - 1 day

### Week 2: Core Features + Circuit Breaker (5 days)

- Day 1-2: Task 21 (Circuit Breaker with Performance Testing) - 2 days (moved from Phase 3)
- Day 3-4: Task 19 (Flush Strategies with Libraries) - 2 days
- Day 5: Task 20 (Eviction Policies) - 1 day

### Week 2.5: Additional Core Features (2.5 days)

- Day 1-2: Task 22-23 (Token Allocation & Priority) - 1.5 days
- Day 2.5: Task 29 (Background Tasks) - 1 day

### Week 3: Testing & Polish (5 days)

- Day 1-3: Task 27 (E2E Integration Tests) - 3 days (high complexity confirmed by consensus)
- Day 4-5: Tasks 25-26, 28 (Other Testing) - 2 days

### Week 3.5: Metrics & Completion (2.5 days)

- Day 1-2: Task 24 (Metrics) - 2 days (expanded due to circuit breaker complexity)
- Day 2.5: Final integration and polish - 0.5 days

### Future Sprint: Advanced Features

- Task 21 (AI Summarizer) - 2 days when needed (renumbered - circuit breaker moved to main timeline)

## Benefits Summary

### Code Reduction

- **Token Management**: ~200 lines → 50 lines
- **Redis Operations**: ~300 lines → 100 lines
- **Circuit Breaker**: ~100 lines → 20 lines
- **Eviction Logic**: ~150 lines → 50 lines
- **Total Reduction**: ~750 lines (75% reduction in new code)

### Quality Improvements

- **Multi-provider support** out of the box
- **Production-grade resilience** patterns
- **Built-in metrics** and monitoring
- **Well-tested algorithms** (LRU, ARC)
- **Reduced maintenance** burden

### Risk Mitigation

- All libraries are production-tested
- Gradual rollout with feature flags
- Existing patterns preserved
- Easy rollback procedures

## Library Dependencies Summary

### Required Libraries (Add to go.mod)

```go
require (
    github.com/open-and-sustainable/alembica v1.0.0 // Multi-provider tokens
    github.com/slok/goresilience v0.2.0             // Resilience patterns
    github.com/hashicorp/golang-lru v1.0.2          // LRU/ARC cache
    github.com/golanguzb70/lrucache v1.2.0         // Token-aware LRU
    github.com/hibiken/asynq v0.24.1               // Background tasks
)
```

### Already Available (No action needed)

- `autoload.ConfigRegistry` - Configuration management
- `pkg/tplengine` - Template processing
- `engine/infra/cache` - Redis & Lock management
- `github.com/pkoukk/tiktoken-go` - Token counting fallback

## Conclusion

By leveraging existing infrastructure and proven libraries, we can:

1. **Reduce implementation time** from 4 weeks to 3.5 weeks (revised after consensus analysis)
2. **Eliminate ~750 lines** of custom code
3. **Add advanced capabilities** (multi-provider support, resilience)
4. **Improve maintainability** with well-tested components
5. **Follow Compozy principles** of reusing proven solutions

**Key Updates After Consensus Analysis:**

- **Distributed systems complexity** properly acknowledged for Tasks 17, 21, 27
- **Performance testing requirements** added for circuit breaker configuration
- **Test environment setup** complexity factored into E2E testing timeline
- **Timeline adjusted** to 3.5 weeks to account for real-world distributed systems challenges

This approach addresses all identified gaps while acknowledging the inherent complexity of distributed systems and ensuring adequate time for proper testing and tuning.
