## status: pending

<task_context>
<domain>sdk/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 14.0: Task: Parallel (S)

## Overview

Implement ParallelBuilder for executing multiple tasks concurrently. Supports wait-all or wait-first semantics.

<critical>
- **MANDATORY** align with engine task type: `TaskTypeParallel = "parallel"`
- **MANDATORY** use context-first Build(ctx) pattern
</critical>

<requirements>
- ParallelBuilder for concurrent task execution
- AddTask method for registering parallel tasks
- WaitAll flag (true = all tasks, false = first completion)
- Error accumulation pattern
</requirements>

## Subtasks

- [ ] 14.1 Create sdk/task/parallel.go
- [ ] 14.2 Implement ParallelBuilder struct and constructor
- [ ] 14.3 Add AddTask method for task registration
- [ ] 14.4 Add WithWaitAll method
- [ ] 14.5 Implement Build(ctx) with validation
- [ ] 14.6 Write unit tests

## Implementation Details

Reference: `tasks/prd-sdk/03-sdk-entities.md` (Section 5.2: Parallel Task)

### Key APIs

```go
// sdk/task/parallel.go
func NewParallel(id string) *ParallelBuilder
func (b *ParallelBuilder) AddTask(taskID string) *ParallelBuilder
func (b *ParallelBuilder) WithWaitAll(waitAll bool) *ParallelBuilder
func (b *ParallelBuilder) Build(ctx context.Context) (*task.Config, error)
```

### Relevant Files

- `sdk/task/parallel.go` - ParallelBuilder implementation
- `engine/task/config.go` - Task config struct

### Dependent Files

- `sdk/internal/errors/build_error.go` - Error aggregation

## Deliverables

- ✅ `sdk/task/parallel.go` with ParallelBuilder
- ✅ Task registration (AddTask)
- ✅ Wait-all vs wait-first configuration
- ✅ Build(ctx) validation
- ✅ Unit tests with table-driven cases

## Tests

Unit tests from `_tests.md`:
- [ ] Parallel task with multiple tasks (wait-all)
- [ ] Parallel task with wait-first semantics
- [ ] Adding tasks incrementally
- [ ] Error: no tasks specified
- [ ] Error: empty task ID
- [ ] Error: duplicate task IDs
- [ ] BuildError aggregation

## Success Criteria

- Builder creates valid `TaskTypeParallel` config
- AddTask accumulates task IDs
- WaitAll flag configures execution mode
- Validation rejects invalid states (no tasks)
- Test coverage ≥95%
- `make lint && make test` pass
