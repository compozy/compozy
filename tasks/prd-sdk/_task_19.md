## status: pending

<task_context>
<domain>sdk/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 19.0: Task: Composite (S)

## Overview

Implement CompositeBuilder for nesting workflows as tasks, enabling workflow composition and reusability.

<critical>
- **MANDATORY** align with engine task type: `TaskTypeComposite = "composite"`
- **MANDATORY** use context-first Build(ctx) pattern
- **MANDATORY** support input mapping to nested workflow
</critical>

<requirements>
- CompositeBuilder for nested workflows
- Workflow reference configuration
- Input mapping to nested workflow
- Error accumulation pattern
</requirements>

## Subtasks

- [ ] 19.1 Create sdk/task/composite.go
- [ ] 19.2 Implement CompositeBuilder struct and constructor
- [ ] 19.3 Add WithWorkflow method for workflow reference
- [ ] 19.4 Add WithInput method for input mapping
- [ ] 19.5 Implement Build(ctx) with validation
- [ ] 19.6 Write unit tests

## Implementation Details

Reference: `tasks/prd-sdk/03-sdk-entities.md` (Section 5.7: Composite Task)

### Key APIs

```go
// sdk/task/composite.go
func NewComposite(id string) *CompositeBuilder
func (b *CompositeBuilder) WithWorkflow(workflowID string) *CompositeBuilder
func (b *CompositeBuilder) WithInput(input map[string]string) *CompositeBuilder
func (b *CompositeBuilder) Build(ctx context.Context) (*task.Config, error)
```

### Relevant Files

- `sdk/task/composite.go` - CompositeBuilder implementation
- `engine/task/config.go` - Task config struct

### Dependent Files

- `sdk/internal/errors/build_error.go` - Error aggregation

## Deliverables

- ✅ `sdk/task/composite.go` with CompositeBuilder
- ✅ Workflow reference configuration
- ✅ Input mapping support
- ✅ Build(ctx) validation
- ✅ Unit tests with table-driven cases

## Tests

Unit tests from `_tests.md`:
- [ ] Composite task with workflow reference
- [ ] Composite task with input mapping
- [ ] Input mapping with templates
- [ ] Error: missing workflow ID
- [ ] Error: empty workflow ID
- [ ] BuildError aggregation

## Success Criteria

- Builder creates valid `TaskTypeComposite` config
- WithWorkflow sets nested workflow reference
- WithInput configures input mapping
- Validation rejects invalid states (missing workflow)
- Test coverage ≥95%
- `make lint && make test` pass
