---
status: completed
---

<task_context>
<domain>engine/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>task_1,task_2,task_4</dependencies>
</task_context>

# Task 5.0: Implement Async-Safe Memory Instance Management

## Overview

Create thread-safe memory instances with distributed locking and async operations. This system orchestrates all memory features (token management, flushing, locking) into a cohesive async-safe interface that supports concurrent agent access while maintaining data consistency.

## Subtasks

- [ ] 6.1 Build AsyncSafeMemoryInstance using existing LockManager from engine/infra/cache
- [ ] 6.2 Integrate with Temporal workflows for async operations
- [ ] 6.3 Create performFlushAsync as Temporal activity for background processing
- [ ] 6.4 Use existing Redis patterns from engine/task/services/redis_store.go
- [ ] 6.5 Implement diagnostic methods following existing health check patterns

## Implementation Details

Build `AsyncSafeMemoryInstance` as the orchestrating component that combines:

- Distributed locking using existing `LockManager` from `engine/infra/cache/lock_manager.go`
- Token-based memory management with tiktoken-go (from Task 2)
- Hybrid flushing strategy using Temporal activities for async processing
- Optimized flush checking using message counts

Key features:

- `Append()` with distributed locking using existing `RedisLockManager`
- Use existing lock patterns with automatic renewal from `lock_manager.go`
- `performFlushAsync()` implemented as Temporal activity following existing patterns
- Follow existing Redis store patterns from `engine/task/services/redis_store.go`
- Memory key template evaluation using existing tplengine
- `MemoryHealth` diagnostics following existing health check patterns

**Integration with Existing Infrastructure**:

- Use `cache.LockManager` interface for distributed locking
- Implement async operations as Temporal activities (see `engine/workflow/activities/`)
- Follow existing Redis patterns with TTL management
- Use existing context propagation and error handling patterns
- Leverage existing monitoring and metrics infrastructure

The system must follow the existing async patterns in the codebase, using Temporal for workflow orchestration rather than introducing new async libraries. This maintains consistency with the current architecture.

# Relevant Files

## Core Implementation Files

- `engine/memory/instance.go` - AsyncSafeMemoryInstance with distributed locking
- `engine/memory/interfaces.go` - Memory interface with async operations
- `engine/memory/activities.go` - Temporal activities for async operations
- `engine/infra/cache/lock_manager.go` - Existing LockManager to use
- `engine/task/services/redis_store.go` - Redis patterns to follow

## Test Files

- `engine/memory/instance_test.go` - Async-safe operations and locking tests
- `engine/memory/activities_test.go` - Temporal activity tests
- `test/integration/memory/concurrent_test.go` - Concurrent access pattern tests

## Success Criteria

- Async-safe operations work correctly under concurrent access patterns
- Distributed locking with existing LockManager prevents data loss
- Token counting with tiktoken-go provides accurate measurements
- Hybrid flushing via Temporal activities integrates with existing patterns
- Optimized flush checking avoids performance bottlenecks
- Memory health reporting follows existing health check patterns
- Template evaluation works with all workflow context variables
- Integration maintains consistency with existing architecture

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
