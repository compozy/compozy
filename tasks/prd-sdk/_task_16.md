## status: completed

<task_context>
<domain>sdk/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 16.0: Task: Router (S)

## Overview

Implement RouterBuilder for conditional routing (switch logic) to execute different tasks based on conditions.

<critical>
- **MANDATORY** align with engine task type: `TaskTypeRouter = "router"`
- **MANDATORY** use context-first Build(ctx) pattern
- **MANDATORY** support default route fallback
</critical>

<requirements>
- RouterBuilder for conditional routing
- Condition evaluation
- AddRoute method for conditional branches
- Default route fallback
- Error accumulation pattern
</requirements>

## Subtasks

- [x] 16.1 Create sdk/task/router.go
- [x] 16.2 Implement RouterBuilder struct and constructor
- [x] 16.3 Add WithCondition method
- [x] 16.4 Add AddRoute method for conditional branches
- [x] 16.5 Add WithDefault method for fallback
- [x] 16.6 Implement Build(ctx) with validation
- [x] 16.7 Write unit tests

## Implementation Details

Reference: `tasks/prd-sdk/03-sdk-entities.md` (Section 5.4: Router Task)

### Key APIs

```go
// sdk/task/router.go
func NewRouter(id string) *RouterBuilder
func (b *RouterBuilder) WithCondition(condition string) *RouterBuilder
func (b *RouterBuilder) AddRoute(condition string, taskID string) *RouterBuilder
func (b *RouterBuilder) WithDefault(taskID string) *RouterBuilder
func (b *RouterBuilder) Build(ctx context.Context) (*task.Config, error)
```

### Relevant Files

- `sdk/task/router.go` - RouterBuilder implementation
- `engine/task/config.go` - Task config struct

### Dependent Files

- `sdk/internal/errors/build_error.go` - Error aggregation

## Deliverables

- ✅ `sdk/task/router.go` with RouterBuilder
- ✅ Condition evaluation support
- ✅ Route registration (AddRoute)
- ✅ Default route fallback
- ✅ Build(ctx) validation
- ✅ Unit tests with table-driven cases

## Tests

Unit tests from `_tests.md`:
- [x] Router task with multiple routes
- [x] Router task with default route
- [x] Adding routes incrementally
- [x] Error: no routes specified
- [x] Error: empty task ID in route
- [x] Error: duplicate conditions
- [x] BuildError aggregation

## Success Criteria

- Builder creates valid `TaskTypeRouter` config
- AddRoute accumulates conditional branches
- WithDefault sets fallback route
- Validation rejects invalid states (no routes)
- Test coverage ≥95%
- `make lint && make test` pass
