---
status: pending
---

<task_context>
<domain>engine/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>lru_libraries</dependencies>
</task_context>

# Task 19.0: Implement Additional Flush Strategies with Library Support

## Overview

Implement LRU, LFU, and Priority-based flush strategies using proven Go libraries instead of custom implementations. This extends the existing FIFO strategy and leverages `github.com/hashicorp/golang-lru` and `github.com/golanguzb70/lrucache` to avoid reinventing complex algorithms.

## Subtasks

- [ ] 19.1 Add library dependencies to go.mod
- [ ] 19.2 Implement LRU strategy using `hashicorp/golang-lru`
- [ ] 19.3 Implement token-cost-aware LRU using `golanguzb70/lrucache`
- [ ] 19.4 Implement priority-based flush strategy
- [ ] 19.5 Add strategy factory and registration system
- [ ] 19.6 Add comprehensive tests for each strategy
- [ ] 19.7 Add performance benchmarks comparing strategies

## Implementation Details

### Library Dependencies

Add to `go.mod`:

```go
require (
    github.com/hashicorp/golang-lru v1.0.2          // ARC algorithm, built-in metrics
    github.com/golanguzb70/lrucache v1.2.0         // Custom cost functions for tokens
)
```

### LRU Strategy Implementation

```go
// engine/memory/instance/strategies/lru_strategy.go
package strategies

import (
    lru "github.com/hashicorp/golang-lru"
    "github.com/compozy/compozy/engine/memory/core"
)

type LRUStrategy struct {
    cache *lru.ARCCache // Adaptive Replacement Cache
    config *core.FlushingStrategyConfig
}

func NewLRUStrategy(config *core.FlushingStrategyConfig) (*LRUStrategy, error) {
    // Use ARC (Adaptive Replacement Cache) for better hit rates
    cache, err := lru.NewARC(config.CacheSize)
    if err != nil {
        return nil, fmt.Errorf("failed to create LRU cache: %w", err)
    }

    return &LRUStrategy{
        cache: cache,
        config: config,
    }, nil
}

func (s *LRUStrategy) ShouldFlush(tokenCount, messageCount int, config *core.Resource) bool {
    // Similar logic to FIFO but considering access patterns
    return s.exceedsThresholds(tokenCount, messageCount, config)
}

func (s *LRUStrategy) PerformFlush(
    ctx context.Context,
    messages []llm.Message,
    config *core.Resource,
) (*core.FlushMemoryActivityOutput, error) {
    // Track message access patterns
    evictedMessages := s.selectLRUMessages(messages, config)

    return &core.FlushMemoryActivityOutput{
        Success:          true,
        SummaryGenerated: false,
        MessageCount:     len(messages) - len(evictedMessages),
        TokenCount:       s.calculateRemainingTokens(messages, evictedMessages),
    }, nil
}
```

### Token-Cost-Aware LRU Strategy

```go
// engine/memory/instance/strategies/token_lru_strategy.go
package strategies

import (
    "github.com/golanguzb70/lrucache"
    "github.com/compozy/compozy/engine/memory/core"
)

type TokenAwareLRUStrategy struct {
    cache  *lrucache.LRUCache
    config *core.FlushingStrategyConfig
}

func NewTokenAwareLRUStrategy(maxTokens int) *TokenAwareLRUStrategy {
    cache := lrucache.NewLRUCache(
        lrucache.WithCapacity(int64(maxTokens)),
        lrucache.WithCostFunc(func(key, value interface{}) int64 {
            if msg, ok := value.(core.MessageWithTokens); ok {
                return int64(msg.TokenCount)
            }
            return 1 // fallback cost
        }),
        lrucache.WithEvictionCallback(func(key, value interface{}) {
            // Optional: log evictions for monitoring
        }),
    )

    return &TokenAwareLRUStrategy{
        cache: cache,
    }
}

func (s *TokenAwareLRUStrategy) PerformFlush(
    ctx context.Context,
    messages []llm.Message,
    config *core.Resource,
) (*core.FlushMemoryActivityOutput, error) {
    // Use token cost for eviction decisions
    evictedMessages := s.evictByTokenCost(messages, config)

    return &core.FlushMemoryActivityOutput{
        Success:          true,
        SummaryGenerated: false,
        MessageCount:     len(messages) - len(evictedMessages),
        TokenCount:       s.calculateTokensAfterEviction(messages, evictedMessages),
    }, nil
}
```

### Strategy Factory

```go
// engine/memory/instance/strategies/factory.go
package strategies

import "github.com/compozy/compozy/engine/memory/core"

type StrategyFactory struct {
    strategies map[core.FlushingStrategyType]func(*core.FlushingStrategyConfig) (instance.FlushStrategy, error)
}

func NewStrategyFactory() *StrategyFactory {
    factory := &StrategyFactory{
        strategies: make(map[core.FlushingStrategyType]func(*core.FlushingStrategyConfig) (instance.FlushStrategy, error)),
    }

    // Register strategies
    factory.Register(core.SimpleFIFOFlushing, func(config *core.FlushingStrategyConfig) (instance.FlushStrategy, error) {
        return NewFIFOStrategy(0.8), nil
    })

    factory.Register(core.LRUFlushing, func(config *core.FlushingStrategyConfig) (instance.FlushStrategy, error) {
        return NewLRUStrategy(config)
    })

    factory.Register(core.TokenAwareLRUFlushing, func(config *core.FlushingStrategyConfig) (instance.FlushStrategy, error) {
        return NewTokenAwareLRUStrategy(config.MaxTokens), nil
    })

    return factory
}
```

**Key Implementation Notes:**

- Uses proven algorithms from established libraries
- ARC (Adaptive Replacement Cache) provides better hit rates than basic LRU
- Token-cost awareness for intelligent eviction based on content size
- Factory pattern for strategy registration and selection
- Comprehensive metrics and monitoring integration

## Success Criteria

- ✅ LRU strategy implemented using `hashicorp/golang-lru` with ARC algorithm
- ✅ Token-aware LRU strategy using `golanguzb70/lrucache` with custom cost functions
- ✅ Priority-based strategy respects message importance levels
- ✅ Strategy factory enables dynamic strategy selection
- ✅ Comprehensive tests validate each strategy's behavior
- ✅ Performance benchmarks show improvement over naive implementations
- ✅ Integration with existing flush workflow works seamlessly
- ✅ Monitoring and metrics capture strategy performance

<critical>
**MANDATORY REQUIREMENTS:**

- **MUST** use specified libraries - no custom LRU/LFU implementations
- **MUST** maintain interface compatibility with existing flush system
- **MUST** include comprehensive test coverage for each strategy
- **MUST** add performance benchmarks comparing strategy efficiency
- **MUST** implement proper error handling and fallback mechanisms
- **MUST** follow established patterns for strategy registration
- **MUST** run `make lint` and `make test` before completion
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for completion
  </critical>
