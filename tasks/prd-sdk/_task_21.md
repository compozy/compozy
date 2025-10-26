## status: pending

<task_context>
<domain>sdk/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>engine/task, sdk/internal</dependencies>
</task_context>

# Task 21.0: Task: Memory (S)

## Overview

Implement the MemoryTaskBuilder in `sdk/task/memory.go` to provide SDK support for memory task operations (read, append, clear). This builder creates task configurations for memory operations within workflows.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Context-first patterns)
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Memory task API section 5.9)
</critical>

<requirements>
- Implement MemoryTaskBuilder with context-first Build(ctx) method
- Support operations: read, append, clear
- Follow error accumulation pattern (BuildError)
- Validate memory reference IDs are non-empty
- All methods must return *MemoryTaskBuilder for fluent API
</requirements>

## Subtasks

- [ ] 21.1 Create sdk/task/memory.go with MemoryTaskBuilder struct
- [ ] 21.2 Implement constructor NewMemoryTask(id string)
- [ ] 21.3 Implement WithOperation, WithMemory, WithContent methods
- [ ] 21.4 Implement Build(ctx) with validation and error aggregation
- [ ] 21.5 Add unit tests for all methods and error cases

## Implementation Details

High-level builder structure per 03-sdk-entities.md section 5.9:

```go
type MemoryTaskBuilder struct {
    config *task.Config
    errors []error
}

// Operations: "read", "append", "clear"
func (b *MemoryTaskBuilder) WithOperation(op string) *MemoryTaskBuilder
func (b *MemoryTaskBuilder) WithMemory(memoryID string) *MemoryTaskBuilder
func (b *MemoryTaskBuilder) WithContent(content string) *MemoryTaskBuilder
func (b *MemoryTaskBuilder) Build(ctx context.Context) (*task.Config, error)
```

Context-first pattern enforcement:
- Build(ctx) must use logger.FromContext(ctx)
- Validation must accept context
- Tests must use t.Context()

### Relevant Files

- `sdk/task/memory.go` (new)
- `sdk/internal/errors/build_error.go` (existing)
- `engine/task/tasks/shared/validation.go` (reference for task types)

### Dependent Files

- `sdk/task/basic.go` (pattern reference)
- `sdk/workflow/builder.go` (consumer)

## Deliverables

- sdk/task/memory.go with complete MemoryTaskBuilder
- Unit tests in sdk/task/memory_test.go with table-driven tests
- Error aggregation for invalid operations and missing memory IDs
- GoDoc comments for all public methods

## Tests

Unit tests from _tests.md (Task builder section):

- [ ] Valid memory task with all fields builds successfully
- [ ] Memory task with empty operation returns BuildError
- [ ] Memory task with empty memoryID returns BuildError
- [ ] Append operation without content returns BuildError
- [ ] Clear operation ignores content field
- [ ] Build(ctx) propagates context to validation
- [ ] Error accumulation for multiple validation failures
- [ ] TaskTypeMemory is set correctly in config

## Success Criteria

- All unit tests pass with >95% coverage
- make lint passes without errors
- Builder follows fluent API pattern consistently
- Error messages are clear and actionable
- Context-first pattern enforced throughout
