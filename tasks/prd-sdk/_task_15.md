## status: completed

<task_context>
<domain>sdk/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 15.0: Task: Collection (S)

## Overview

Implement CollectionBuilder for iterating over collections (arrays) and executing a task for each item.

<critical>
- **MANDATORY** align with engine task type: `TaskTypeCollection = "collection"`
- **MANDATORY** use context-first Build(ctx) pattern
</critical>

<requirements>
- CollectionBuilder for iteration
- Collection source specification (template expression)
- Task to execute per item
- Item variable name configuration (default: "item")
- Error accumulation pattern
</requirements>

## Subtasks

- [x] 15.1 Create sdk/task/collection.go
- [x] 15.2 Implement CollectionBuilder struct and constructor
- [x] 15.3 Add WithCollection method for source
- [x] 15.4 Add WithTask method for iteration task
- [x] 15.5 Add WithItemVar method for variable naming
- [x] 15.6 Implement Build(ctx) with validation
- [x] 15.7 Write unit tests

## Implementation Details

Reference: `tasks/prd-sdk/03-sdk-entities.md` (Section 5.3: Collection Task)

### Key APIs

```go
// sdk/task/collection.go
func NewCollection(id string) *CollectionBuilder
func (b *CollectionBuilder) WithCollection(collection string) *CollectionBuilder
func (b *CollectionBuilder) WithTask(taskID string) *CollectionBuilder
func (b *CollectionBuilder) WithItemVar(varName string) *CollectionBuilder
func (b *CollectionBuilder) Build(ctx context.Context) (*task.Config, error)
```

### Relevant Files

- `sdk/task/collection.go` - CollectionBuilder implementation
- `engine/task/config.go` - Task config struct

### Dependent Files

- `sdk/internal/errors/build_error.go` - Error aggregation

## Deliverables

- ✅ `sdk/task/collection.go` with CollectionBuilder
- ✅ Collection source configuration
- ✅ Task to execute per item
- ✅ Item variable naming
- ✅ Build(ctx) validation
- ✅ Unit tests with table-driven cases

## Tests

Unit tests from `_tests.md`:
- [x] Collection task with default item var
- [x] Collection task with custom item var
- [x] Collection source from template
- [x] Error: missing collection source
- [x] Error: missing task
- [x] Error: empty task ID
- [x] BuildError aggregation

## Success Criteria

- Builder creates valid `TaskTypeCollection` config
- Collection source preserved as template
- Task ID registered for iteration
- Item variable defaults to "item"
- Validation rejects invalid states
- Test coverage ≥95%
- `make lint && make test` pass
