## status: pending

<task_context>
<domain>sdk/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 18.0: Task: Aggregate (S)

## Overview

Implement AggregateBuilder for combining results from multiple tasks using aggregation strategies (concat, merge, custom).

<critical>
- **MANDATORY** align with engine task type: `TaskTypeAggregate = "aggregate"`
- **MANDATORY** use context-first Build(ctx) pattern
- **MANDATORY** support multiple aggregation strategies
</critical>

<requirements>
- AggregateBuilder for result aggregation
- AddTask method for task registration
- Strategy configuration (concat, merge, custom)
- Custom aggregation function support
- Error accumulation pattern
</requirements>

## Subtasks

- [ ] 18.1 Create sdk/task/aggregate.go
- [ ] 18.2 Implement AggregateBuilder struct and constructor
- [ ] 18.3 Add AddTask method for task registration
- [ ] 18.4 Add WithStrategy method for aggregation mode
- [ ] 18.5 Add WithFunction method for custom functions
- [ ] 18.6 Implement Build(ctx) with validation
- [ ] 18.7 Write unit tests

## Implementation Details

Reference: `tasks/prd-sdk/03-sdk-entities.md` (Section 5.6: Aggregate Task)

### Key APIs

```go
// sdk/task/aggregate.go
func NewAggregate(id string) *AggregateBuilder
func (b *AggregateBuilder) AddTask(taskID string) *AggregateBuilder
func (b *AggregateBuilder) WithStrategy(strategy string) *AggregateBuilder  // "concat", "merge", "custom"
func (b *AggregateBuilder) WithFunction(fn string) *AggregateBuilder
func (b *AggregateBuilder) Build(ctx context.Context) (*task.Config, error)
```

### Relevant Files

- `sdk/task/aggregate.go` - AggregateBuilder implementation
- `engine/task/config.go` - Task config struct

### Dependent Files

- `sdk/internal/errors/build_error.go` - Error aggregation

## Deliverables

- ✅ `sdk/task/aggregate.go` with AggregateBuilder
- ✅ Task registration (AddTask)
- ✅ Strategy configuration (concat, merge, custom)
- ✅ Custom function support
- ✅ Build(ctx) validation
- ✅ Unit tests with table-driven cases

## Tests

Unit tests from `_tests.md`:
- [ ] Aggregate task with concat strategy
- [ ] Aggregate task with merge strategy
- [ ] Aggregate task with custom function
- [ ] Adding tasks incrementally
- [ ] Error: no tasks specified
- [ ] Error: invalid strategy
- [ ] Error: custom strategy without function
- [ ] BuildError aggregation

## Success Criteria

- Builder creates valid `TaskTypeAggregate` config
- AddTask accumulates task IDs
- WithStrategy sets aggregation mode
- WithFunction enables custom aggregation
- Validation rejects invalid states
- Test coverage ≥95%
- `make lint && make test` pass
