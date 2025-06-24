# Implementation Gap Tasks for Memory Engine

## Overview

This document tracks the implementation tasks for addressing gaps identified in the memory engine implementation analysis. Each task includes specific implementation details and library recommendations.

## Task Status Updates

### From Original Task List

- **Task 1.0**: Implement Enhanced Memory Domain Foundation - **COMPLETED**

    - ✅ 1.1: Memory interfaces defined
    - ✅ 1.2: Data models implemented
    - ✅ 1.3: Redis extensions completed
    - ✅ 1.4: Lock manager wrapper created
    - ✅ 1.5: Tests implemented

- **Task 2.0**: Implement Token Management and Flushing System - **COMPLETED**

    - ✅ Core token management implemented
    - ❌ Missing: Additional flush strategies (LRU, LFU, Priority-based)

- **Task 3.0**: Create Fixed Configuration Resolution System - **COMPLETED**
    - ❌ Missing: Registry integration
    - ❌ Missing: Template engine integration

## New Implementation Tasks

### Task 15.0: Complete Configuration Loading Implementation

**Priority**: HIGH  
**Complexity**: LOW (2/10)  
**Dependencies**: None (uses existing infrastructure)

#### Subtasks

- [ ] 15.1: Implement `loadMemoryConfig` in `config_resolver.go`
- [ ] 15.2: Add unit tests for config loading
- [ ] 15.3: Add integration tests with registry

#### Implementation Details

```go
// Uses existing autoload.ConfigRegistry
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

**Libraries/Dependencies**: None - uses existing `autoload.ConfigRegistry`

### Task 16.0: Complete Template Engine Integration

**Priority**: HIGH  
**Complexity**: LOW (2/10)  
**Dependencies**: None (uses existing infrastructure)

#### Subtasks

- [ ] 16.1: Implement template evaluation in `resolveMemoryKey`
- [ ] 16.2: Add unit tests for template resolution
- [ ] 16.3: Add edge case handling (invalid templates, missing vars)

#### Implementation Details

```go
// Uses existing pkg/tplengine
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
    sanitizedKey := mm.sanitizeKey(result.Text)
    projectIDVal := extractProjectID(workflowContextData)
    return sanitizedKey, projectIDVal
}
```

**Libraries/Dependencies**: None - uses existing `pkg/tplengine`

### Task 17.0: Implement Distributed Lock Manager

**Priority**: HIGH  
**Complexity**: MEDIUM (5/10)  
**Dependencies**: Redis

#### Subtasks

- [ ] 17.1: Replace dummy lock implementation in `instance_builder.go`
- [ ] 17.2: Implement proper lock/unlock with existing `LockManager`
- [ ] 17.3: Add timeout and retry logic
- [ ] 17.4: Add comprehensive concurrency tests

#### Implementation Details

```go
// Use existing engine/infra/cache/lock_manager.go
type lockManagerAdapter struct {
    lockManager *cache.LockManager
    locks       sync.Map // track active locks
}

func (lma *lockManagerAdapter) Lock(ctx context.Context, key string, ttl time.Duration) (instance.Lock, error) {
    lock, err := lma.lockManager.AcquireLock(ctx, key, ttl)
    if err != nil {
        return nil, err
    }
    lma.locks.Store(key, lock)
    return &distributedLock{key: key, manager: lma}, nil
}
```

**Libraries/Dependencies**: None - uses existing `engine/infra/cache/lock_manager.go`

### Task 18.0: Implement Error Logging for Ignored Errors

**Priority**: HIGH  
**Complexity**: LOW (1/10)  
**Dependencies**: None

#### Subtasks

- [ ] 18.1: Add logging to `flush_operations.go` line 148
- [ ] 18.2: Add logging to `memory_instance.go` line 89
- [ ] 18.3: Review for other ignored errors

#### Implementation Details

```go
// flush_operations.go
if err := f.markFlushPending(ctx, false); err != nil {
    f.log.Error("Failed to clear flush pending flag during cleanup",
        "error", err,
        "memory_key", f.instanceKey)
}

// memory_instance.go
if err := lock(); err != nil {
    mi.log.Error("Failed to release lock",
        "error", err,
        "operation", "clear",
        "memory_id", mi.id)
}
```

**Libraries/Dependencies**: None - uses existing logger

### Task 19.0: Implement Additional Flush Strategies

**Priority**: MEDIUM  
**Complexity**: HIGH (7/10)  
**Dependencies**: None

#### Subtasks

- [ ] 19.1: Implement LRU (Least Recently Used) strategy
- [ ] 19.2: Implement LFU (Least Frequently Used) strategy
- [ ] 19.3: Implement Priority-based flush strategy
- [ ] 19.4: Add strategy factory and registration
- [ ] 19.5: Add comprehensive tests for each strategy

#### Implementation Details

```go
// LRU Strategy
type LRUStrategy struct {
    accessTimes sync.Map // message ID -> last access time
}

// LFU Strategy
type LFUStrategy struct {
    frequencies sync.Map // message ID -> access count
}

// Priority Strategy (already has foundation in token_manager.go)
type PriorityStrategy struct {
    priorityExtractor func(msg llm.Message) int
}
```

**Libraries/Dependencies**:

- Consider `github.com/hashicorp/golang-lru` for LRU cache implementation
- Or implement custom using existing patterns

### Task 20.0: Implement Eviction Policies

**Priority**: MEDIUM  
**Complexity**: MEDIUM (5/10)  
**Dependencies**: Task 19.0

#### Subtasks

- [ ] 20.1: Implement FIFO eviction policy
- [ ] 20.2: Implement LRU eviction policy
- [ ] 20.3: Implement priority-based eviction
- [ ] 20.4: Integrate with flush strategies

#### Implementation Details

```go
// Implement the EvictionPolicy interface
type FIFOEvictionPolicy struct{}

func (p *FIFOEvictionPolicy) SelectMessagesToEvict(
    messages []llm.Message,
    targetCount int,
) []llm.Message {
    if len(messages) <= targetCount {
        return nil
    }
    return messages[:len(messages)-targetCount]
}
```

**Libraries/Dependencies**: None - implement using existing patterns

### Task 21.0: Implement AI-Based Summarizer

**Priority**: LOW  
**Complexity**: HIGH (8/10)  
**Dependencies**: LLM integration

#### Subtasks

- [ ] 21.1: Design AI summarizer interface
- [ ] 21.2: Implement OpenAI-based summarizer
- [ ] 21.3: Implement fallback to rule-based
- [ ] 21.4: Add quality metrics for summaries

#### Implementation Details

```go
type AIBasedSummarizer struct {
    llmClient    LLMClient
    fallback     MessageSummarizer
    maxRetries   int
}

func (s *AIBasedSummarizer) SummarizeMessages(
    ctx context.Context,
    messages []memcore.MessageWithTokens,
    targetTokens int,
) (llm.Message, []memcore.MessageWithTokens, []memcore.MessageWithTokens, error) {
    // Use LLM to generate intelligent summary
    prompt := buildSummaryPrompt(messages, targetTokens)
    summary, err := s.llmClient.Complete(ctx, prompt)
    if err != nil {
        return s.fallback.SummarizeMessages(ctx, messages, targetTokens)
    }
    // Parse and return summary
}
```

**Libraries/Dependencies**: Uses existing LLM integration

### Task 22.0: Complete Token Allocation System

**Priority**: MEDIUM  
**Complexity**: MEDIUM (6/10)  
**Dependencies**: None

#### Subtasks

- [ ] 22.1: Implement allocation-aware eviction in `ApplyTokenAllocation`
- [ ] 22.2: Add message categorization (system/short_term/long_term)
- [ ] 22.3: Implement budget enforcement per category
- [ ] 22.4: Add tests for various allocation scenarios

#### Implementation Details

```go
func (tmm *TokenMemoryManager) ApplyTokenAllocation(
    ctx context.Context,
    messages []memcore.MessageWithTokens,
    currentTotalTokens int,
) ([]memcore.MessageWithTokens, int, error) {
    // Categorize messages
    categories := categorizeMessages(messages)

    // Calculate budgets based on allocation ratios
    budgets := calculateBudgets(tmm.config.TokenAllocation, currentTotalTokens)

    // Apply eviction per category
    result := applyBudgetEviction(categories, budgets)

    return result.messages, result.totalTokens, nil
}
```

**Libraries/Dependencies**: None - uses existing structures

### Task 23.0: Integrate Priority-Based Eviction

**Priority**: MEDIUM  
**Complexity**: LOW (3/10)  
**Dependencies**: Task 22.0

#### Subtasks

- [ ] 23.1: Connect `EnforceLimitsWithPriority` to main flow
- [ ] 23.2: Add priority extraction from message metadata
- [ ] 23.3: Add configuration for priority-based eviction
- [ ] 23.4: Add integration tests

**Libraries/Dependencies**: None - implementation already exists

### Task 24.0: Complete Metrics Implementation

**Priority**: MEDIUM  
**Complexity**: MEDIUM (4/10)  
**Dependencies**: Prometheus

#### Subtasks

- [ ] 24.1: Complete Prometheus metric registration
- [ ] 24.2: Add custom metric exporters
- [ ] 24.3: Add detailed performance metrics
- [ ] 24.4: Create Grafana dashboard

**Libraries/Dependencies**:

- Uses existing Prometheus integration
- No new dependencies needed

### Task 25.0: Registry Integration Testing

**Priority**: HIGH  
**Complexity**: LOW (2/10)  
**Dependencies**: Task 15.0

#### Subtasks

- [ ] 25.1: Add integration tests for config loading
- [ ] 25.2: Test registry with multiple memory resources
- [ ] 25.3: Test error scenarios (missing config, invalid type)

**Libraries/Dependencies**: None - uses existing test infrastructure

### Task 26.0: Template Engine Integration Testing

**Priority**: HIGH  
**Complexity**: LOW (2/10)  
**Dependencies**: Task 16.0

#### Subtasks

- [ ] 26.1: Add integration tests for template resolution
- [ ] 26.2: Test complex templates with nested data
- [ ] 26.3: Test error scenarios (invalid template, missing data)

**Libraries/Dependencies**: None - uses existing test infrastructure

### Task 27.0: End-to-End Integration Tests

**Priority**: HIGH  
**Complexity**: MEDIUM (5/10)  
**Dependencies**: Tasks 15-18

#### Subtasks

- [ ] 27.1: Create E2E test with Temporal workflow
- [ ] 27.2: Test concurrent memory access scenarios
- [ ] 27.3: Test flush and cleanup workflows
- [ ] 27.4: Performance benchmarks

**Libraries/Dependencies**:

- Uses existing Temporal test infrastructure
- May use `github.com/stretchr/testify` for assertions

### Task 28.0: Concurrent Access Testing

**Priority**: HIGH  
**Complexity**: MEDIUM (4/10)  
**Dependencies**: Task 17.0

#### Subtasks

- [ ] 28.1: Create concurrent append tests
- [ ] 28.2: Create concurrent read/write tests
- [ ] 28.3: Test distributed lock behavior
- [ ] 28.4: Add race condition detection

**Libraries/Dependencies**:

- Use Go's built-in race detector (`go test -race`)
- Existing test infrastructure

## Implementation Order

### Phase 1: Critical Fixes (Week 1)

1. Task 15.0: Configuration Loading
2. Task 16.0: Template Integration
3. Task 17.0: Distributed Locking
4. Task 18.0: Error Logging

### Phase 2: Core Features (Week 2)

5. Task 19.0: Flush Strategies
6. Task 20.0: Eviction Policies
7. Task 22.0: Token Allocation
8. Task 23.0: Priority Eviction

### Phase 3: Testing & Quality (Week 3)

9. Task 25.0: Registry Testing
10. Task 26.0: Template Testing
11. Task 27.0: E2E Tests
12. Task 28.0: Concurrent Tests

### Phase 4: Advanced Features (Week 4)

13. Task 21.0: AI Summarizer
14. Task 24.0: Metrics Completion

## Key Implementation Notes

1. **No New Dependencies**: Most tasks can be completed using existing infrastructure
2. **Consistent Patterns**: Follow existing error handling, logging, and testing patterns
3. **Incremental Delivery**: Each task is independently valuable
4. **Test Coverage**: Maintain >85% coverage as per project standards

## Success Criteria

- All TODO comments removed or implemented
- All placeholder implementations replaced
- Comprehensive test coverage for new features
- Performance benchmarks meet targets (<50ms overhead)
- No race conditions in concurrent access
- Full integration with existing infrastructure
