## status: completed

<task_context>
<domain>sdk/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 17.0: Task: Wait (S)

## Overview

Implement WaitBuilder for delaying workflow execution by duration or until a condition is met.

<critical>
- **MANDATORY** align with engine task type: `TaskTypeWait = "wait"`
- **MANDATORY** use context-first Build(ctx) pattern
- **MANDATORY** support both duration and condition-based waiting
</critical>

<requirements>
- WaitBuilder for delay operations
- Fixed duration waiting
- Condition-based waiting
- Timeout for condition waiting
- Error accumulation pattern
</requirements>

## Subtasks

- [x] 17.1 Create sdk/task/wait.go
- [x] 17.2 Implement WaitBuilder struct and constructor
- [x] 17.3 Add WithDuration method for fixed delay
- [x] 17.4 Add WithCondition method for conditional wait
- [x] 17.5 Add WithTimeout method for max wait time
- [x] 17.6 Implement Build(ctx) with validation
- [x] 17.7 Write unit tests

## Implementation Details

Reference: `tasks/prd-sdk/03-sdk-entities.md` (Section 5.5: Wait Task)

### Key APIs

```go
// sdk/task/wait.go
func NewWait(id string) *WaitBuilder
func (b *WaitBuilder) WithDuration(duration time.Duration) *WaitBuilder
func (b *WaitBuilder) WithCondition(condition string) *WaitBuilder
func (b *WaitBuilder) WithTimeout(timeout time.Duration) *WaitBuilder
func (b *WaitBuilder) Build(ctx context.Context) (*task.Config, error)
```

### Relevant Files

- `sdk/task/wait.go` - WaitBuilder implementation
- `engine/task/config.go` - Task config struct

### Dependent Files

- `sdk/internal/errors/build_error.go` - Error aggregation

## Deliverables

- ✅ `sdk/task/wait.go` with WaitBuilder
- ✅ Duration-based waiting
- ✅ Condition-based waiting
- ✅ Timeout configuration
- ✅ Build(ctx) validation
- ✅ Unit tests with table-driven cases

## Tests

Unit tests from `_tests.md`:
- [ ] Wait task with fixed duration
- [ ] Wait task with condition
- [ ] Wait task with condition + timeout
- [ ] Error: both duration and condition specified
- [ ] Error: neither duration nor condition
- [ ] Error: negative duration
- [ ] BuildError aggregation

## Success Criteria

- Builder creates valid `TaskTypeWait` config
- WithDuration sets fixed delay
- WithCondition enables conditional waiting
- WithTimeout sets max wait time
- Validation rejects invalid states
- Test coverage ≥95%
- `make lint && make test` pass
