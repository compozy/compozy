## status: pending

<task_context>
<domain>sdk/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>sdk/memory/config.go, engine/memory</dependencies>
</task_context>

# Task 35.0: Memory: Persistence + Token Counting (S)

## Overview

Extend memory ConfigBuilder with persistence backend and token counting configuration.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md
</critical>

<requirements>
- Persistence backend configuration (Redis, Postgres, etc.)
- Token counter provider/model configuration
- Distributed locking support
- Context-first validation
- Error accumulation
</requirements>

## Subtasks

- [ ] 35.1 Add WithPersistence(backend memory.PersistenceBackend) method
- [ ] 35.2 Add WithTokenCounter(provider, model string) method
- [ ] 35.3 Add WithDistributedLocking(enabled bool) method
- [ ] 35.4 Update Build(ctx) validation for persistence settings
- [ ] 35.5 Add unit tests for persistence and token counting

## Implementation Details

Reference from 03-sdk-entities.md section 7.1:

```go
// Persistence backend
func (b *ConfigBuilder) WithPersistence(backend memory.PersistenceBackend) *ConfigBuilder

// Token counting
func (b *ConfigBuilder) WithTokenCounter(provider, model string) *ConfigBuilder

// Distributed locking (for concurrent access)
func (b *ConfigBuilder) WithDistributedLocking(enabled bool) *ConfigBuilder
```

Engine persistence backends from engine/memory:
- PersistenceInMemory (default)
- PersistenceRedis
- PersistencePostgres

Example from architecture:
```go
memory.New("customer-support").
    WithPersistence(memory.PersistenceRedis).
    WithTokenCounter("openai", "gpt-4o-mini").
    WithDistributedLocking(true)
```

### Relevant Files

- `sdk/memory/config.go` (extend existing)
- `engine/memory/types.go` (persistence backend types)

### Dependent Files

- Task 31.0 output (memory ConfigBuilder base)
- Future advanced memory examples

## Deliverables

- Persistence backend method in ConfigBuilder
- Token counter method in ConfigBuilder
- Distributed locking method in ConfigBuilder
- Validation for persistence settings
- Updated package documentation

## Tests

Unit tests mapped from `_tests.md`:

- [ ] WithPersistence sets backend correctly
- [ ] WithTokenCounter sets provider/model
- [ ] WithDistributedLocking enables/disables locking
- [ ] Build(ctx) validates persistence backend values
- [ ] Build(ctx) validates token counter requirements
- [ ] Error cases: invalid backend, missing provider/model
- [ ] Edge cases: in-memory persistence (no locking needed)

## Success Criteria

- Persistence and token counting methods follow builder pattern
- All unit tests pass
- make lint and make test pass
- Memory ConfigBuilder feature-complete per SDK entities
