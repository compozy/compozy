---
status: pending
---

<task_context>
<domain>engine/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>task_1,task_2,task_3</dependencies>
</task_context>

# Task 6.0: Implement Memory Instance with Existing Infrastructure

## Overview

Create thread-safe memory instances using the existing `cache.LockManager` for distributed locking and Temporal for async operations. This follows the established patterns in the codebase where async operations are implemented as Temporal activities rather than using external job queues.

## Subtasks

- [ ] 6.1 Create MemoryInstance using existing cache.LockManager interface
- [ ] 6.2 Implement memory operations with distributed locking patterns
- [ ] 6.3 Create Temporal activities for async flush operations
- [ ] 6.4 Follow Redis patterns from engine/task/services/redis_store.go
- [ ] 6.5 Add health checks following existing patterns

## Implementation Details

**1. Memory Instance Structure:**

```go
type MemoryInstance struct {
    lockManager cache.LockManager  // Use existing interface
    redis       *cache.Redis       // Follow existing Redis patterns
    config      *Config
    projectID   string
    // ... other fields
}
```

**2. Distributed Locking Pattern (from existing code):**

```go
func (m *MemoryInstance) Append(ctx context.Context, messages []Message) error {
    // Acquire lock using existing LockManager
    lock, err := m.lockManager.Acquire(ctx, m.getLockKey(), 30*time.Second)
    if err != nil {
        return fmt.Errorf("failed to acquire lock: %w", err)
    }
    defer lock.Release(ctx)

    // Perform append operation
    // Check flush conditions
    // Trigger Temporal workflow if needed
}
```

**3. Temporal Activity for Async Flush:**

```go
// In engine/memory/activities/flush.go
type FlushMemoryInput struct {
    ProjectID  string
    MemoryID   string
    Strategy   string
}

func (a *FlushMemory) Run(ctx context.Context, input *FlushMemoryInput) error {
    // Implement flush logic following CompleteWorkflow pattern
    activity.RecordHeartbeat(ctx, "Flushing memory")
    // ... flush implementation
}
```

**4. Redis Patterns (following redis_store.go):**

- Use key prefixes for namespacing
- Implement TTL management
- Follow existing error handling patterns
- Use atomic operations where needed

**5. Health Checks:**

```go
func (m *MemoryInstance) HealthCheck(ctx context.Context) error {
    // Follow existing health check patterns
    return m.redis.HealthCheck(ctx)
}
```

The implementation reuses existing infrastructure:

- `cache.LockManager` for distributed locking (extends existing implementation)
- Temporal activities for async operations (follows existing patterns)
- Existing Redis patterns with proper key management
- Standard error handling with `core.NewError`

# Relevant Files

## Core Implementation Files

- `engine/memory/instance.go` - MemoryInstance with distributed locking
- `engine/memory/interfaces.go` - Memory interface with async operations
- `engine/memory/activities/flush.go` - Temporal activities for async operations
- `engine/infra/cache/lock_manager.go` - Existing LockManager to use
- `engine/task/services/redis_store.go` - Redis patterns to follow

## Test Files

- `engine/memory/instance_test.go` - Async-safe operations and locking tests
- `engine/memory/activities/flush_test.go` - Temporal activity tests
- `test/integration/memory/concurrent_test.go` - Concurrent access pattern tests

## Success Criteria

- Async-safe operations work correctly under concurrent access patterns
- Distributed locking with existing LockManager prevents data loss
- Token counting with tiktoken-go provides accurate measurements
- Flush operations via Temporal activities integrate with existing patterns
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
