## status: pending

<task_context>
<domain>v2/task</domain>
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

- [ ] 16.1 Create v2/task/router.go
- [ ] 16.2 Implement RouterBuilder struct and constructor
- [ ] 16.3 Add WithCondition method
- [ ] 16.4 Add AddRoute method for conditional branches
- [ ] 16.5 Add WithDefault method for fallback
- [ ] 16.6 Implement Build(ctx) with validation
- [ ] 16.7 Write unit tests

## Implementation Details

Reference: `tasks/prd-modules/03-sdk-entities.md` (Section 5.4: Router Task)

### Key APIs

```go
// v2/task/router.go
func NewRouter(id string) *RouterBuilder
func (b *RouterBuilder) WithCondition(condition string) *RouterBuilder
func (b *RouterBuilder) AddRoute(condition string, taskID string) *RouterBuilder
func (b *RouterBuilder) WithDefault(taskID string) *RouterBuilder
func (b *RouterBuilder) Build(ctx context.Context) (*task.Config, error)
```

### Relevant Files

- `v2/task/router.go` - RouterBuilder implementation
- `engine/task/config.go` - Task config struct

### Dependent Files

- `v2/internal/errors/build_error.go` - Error aggregation

## Deliverables

- ✅ `v2/task/router.go` with RouterBuilder
- ✅ Condition evaluation support
- ✅ Route registration (AddRoute)
- ✅ Default route fallback
- ✅ Build(ctx) validation
- ✅ Unit tests with table-driven cases

## Tests

Unit tests from `_tests.md`:
- [ ] Router task with multiple routes
- [ ] Router task with default route
- [ ] Adding routes incrementally
- [ ] Error: no routes specified
- [ ] Error: empty task ID in route
- [ ] Error: duplicate conditions
- [ ] BuildError aggregation

## Success Criteria

- Builder creates valid `TaskTypeRouter` config
- AddRoute accumulates conditional branches
- WithDefault sets fallback route
- Validation rejects invalid states (no routes)
- Test coverage ≥95%
- `make lint && make test` pass
