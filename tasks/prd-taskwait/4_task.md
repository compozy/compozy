---
status: pending
---

<task_context>
<domain>engine/infra/store</domain>
<type>implementation</type>
<scope>middleware</scope>
<complexity>medium</complexity>
<dependencies>redis</dependencies>
</task_context>

# Task 4.0: Implement Redis Signal Storage

## Overview

Create Redis-based signal storage for deduplication with proper resource management. This component prevents duplicate signal processing and provides persistence for signal tracking.

## Subtasks

- [ ] 4.1 Create RedisSignalStorage struct with Redis client integration
- [ ] 4.2 Implement NewRedisSignalStorage constructor with TTL configuration
- [ ] 4.3 Implement IsDuplicate method for signal deduplication checking
- [ ] 4.4 Implement MarkProcessed method to record processed signals
- [ ] 4.5 Implement Close method for proper resource cleanup
- [ ] 4.6 Add proper error handling and key naming conventions

## Implementation Details

Implement RedisSignalStorage with proper resource management:

```go
type RedisSignalStorage struct {
    client redis.Client
    ttl    time.Duration
}

func NewRedisSignalStorage(client redis.Client, ttl time.Duration) *RedisSignalStorage {
    if ttl == 0 {
        ttl = 24 * time.Hour // Default TTL
    }
    return &RedisSignalStorage{
        client: client,
        ttl:    ttl,
    }
}

func (r *RedisSignalStorage) IsDuplicate(ctx context.Context, signalID string) (bool, error) {
    key := fmt.Sprintf("wait_signal:%s", signalID)
    result, err := r.client.Exists(ctx, key).Result()
    if err != nil {
        return false, fmt.Errorf("failed to check signal duplicate: %w", err)
    }
    return result > 0, nil
}

func (r *RedisSignalStorage) MarkProcessed(ctx context.Context, signalID string) error {
    key := fmt.Sprintf("wait_signal:%s", signalID)
    err := r.client.SetEX(ctx, key, "processed", r.ttl).Err()
    if err != nil {
        return fmt.Errorf("failed to mark signal as processed: %w", err)
    }
    return nil
}
```

Key requirements:

- Use existing Redis client from engine/infra/store
- Implement proper key naming pattern: `wait_signal:{signalID}`
- Handle TTL configuration with sensible defaults
- Provide proper error wrapping and context handling

## Success Criteria

- [ ] Redis client properly integrated with existing infrastructure
- [ ] Key naming follows consistent pattern across the application
- [ ] TTL configuration prevents unbounded growth
- [ ] Error handling provides meaningful context
- [ ] Resource cleanup properly implemented in Close method
- [ ] Context propagation works correctly for timeouts

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
