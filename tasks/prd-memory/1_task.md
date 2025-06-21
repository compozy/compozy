---
status: pending
---

<task_context>
<domain>engine/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>redis</dependencies>
</task_context>

# Task 1.0: Implement Enhanced Memory Domain Foundation

## Overview

Create the core memory interfaces, data models, and extend existing Redis infrastructure with memory-specific operations. This establishes the foundation for the entire memory system with enhanced features including token management, hybrid flushing, and leveraging existing distributed locking capabilities.

## Subtasks

- [ ] 1.1 Define Memory, MemoryStore interfaces with async operations
- [ ] 1.2 Implement MemoryResource, TokenAllocation, and FlushingStrategy data models
- [ ] 1.3 Extend existing `engine/infra/cache/redis.go` with memory-specific methods
- [ ] 1.4 Create memory-specific wrapper around existing `engine/infra/cache/lock_manager.go`
- [ ] 1.5 Add comprehensive unit and integration tests for all interfaces

## Implementation Details

Define the core `Memory` interface with async operations as the primary interface:

```go
type Memory interface {
    Append(ctx context.Context, msg llm.Message) error
    Read(ctx context.Context) ([]llm.Message, error)
    Len(ctx context.Context) (int, error)
    GetTokenCount(ctx context.Context) (int, error)
    GetMemoryHealth(ctx context.Context) (*MemoryHealth, error)
}
```

Implement MemoryStore interface for persistence-agnostic operations. Create enhanced data models supporting token allocation ratios and flushing strategies.

**Key Architecture Decisions**:

- **REUSE EXISTING**: Extend `engine/infra/cache/redis.go` RedisInterface with memory-specific operations
- **LEVERAGE EXISTING**: Use existing `engine/infra/cache/lock_manager.go` LockManager with memory-specific wrapper
- **NO NEW REDIS**: Do not create new Redis client implementations - extend existing infrastructure
- **NO REDSYNC**: Use existing LockManager with Lua scripts for distributed locking
- Follow existing patterns for Redis operations (Pipeline, Lua scripts)
- Integrate **tiktoken-go** (`github.com/pkoukk/tiktoken-go`) for accurate token counting

# Relevant Files

## Core Implementation Files

- `engine/memory/interfaces.go` - Core Memory, MemoryStore interfaces
- `engine/memory/types.go` - MemoryResource, TokenAllocation, FlushingStrategy data models
- `engine/memory/store.go` - Memory-specific store wrapper using existing Redis infrastructure
- `engine/memory/lock.go` - Memory-specific lock wrapper around existing LockManager

## Existing Infrastructure to Extend

- `engine/infra/cache/redis.go` - Extend RedisInterface with memory-specific operations
- `engine/infra/cache/lock_manager.go` - Use existing distributed locking with Lua scripts

## Test Files

- `engine/memory/interfaces_test.go` - Interface tests with in-memory fakes
- `test/integration/memory/redis_test.go` - Redis integration tests

## Success Criteria

- All core interfaces defined with async operations support
- Enhanced data models support token management and hybrid flushing
- Extended Redis infrastructure supports memory-specific operations
- Existing distributed locking works for memory operations
- Token counting integrated with tiktoken-go for accurate measurements
- Unit tests with in-memory fake implementations achieve >85% coverage
- Integration tests with Redis validate concurrent access patterns
- Lock acquire/release cycles work correctly under load with existing LockManager

<critical>
**MANDATORY REQUIREMENTS:**

- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing parent tasks
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
