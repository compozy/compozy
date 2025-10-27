## status: completed

<task_context>
<domain>sdk/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>sdk/internal/errors, engine/memory</dependencies>
</task_context>

# Task 32.0: Memory: Reference (S)

## Overview

Implement the memory reference builder (`sdk/memory/reference.go`) for attaching memory to agents with key templates.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md
</critical>

<requirements>
- Memory reference builder with key template support
- Context-first Build(ctx) pattern
- Error accumulation following BuildError pattern
- Produces engine/memory.ReferenceConfig
</requirements>

## Subtasks

- [x] 32.1 Create sdk/memory/reference.go with ReferenceBuilder struct
- [x] 32.2 Implement NewReference(memoryID string) constructor
- [x] 32.3 Implement WithKey(keyTemplate string) method
- [x] 32.4 Implement Build(ctx) with validation
- [x] 32.5 Add unit tests for reference builder

## Implementation Details

Reference from 03-sdk-entities.md section 7.2:

```go
type ReferenceBuilder struct {
    config *memory.ReferenceConfig
    errors []error
}

func NewReference(memoryID string) *ReferenceBuilder
func (b *ReferenceBuilder) WithKey(keyTemplate string) *ReferenceBuilder
func (b *ReferenceBuilder) Build(ctx context.Context) (*memory.ReferenceConfig, error)
```

Key template example: `"conversation-{{.conversation.id}}"`

### Relevant Files

- `sdk/memory/reference.go` (new)
- `sdk/internal/errors/build_error.go` (existing)
- `engine/memory/types.go` (engine types)

### Dependent Files

- `sdk/agent/builder.go` (will use memory references)
- Future agent attachment examples

## Deliverables

- `sdk/memory/reference.go` implementing ReferenceBuilder
- Constructor and fluent methods with error accumulation
- Build(ctx) producing engine memory.ReferenceConfig
- Package documentation

## Tests

Unit tests mapped from `_tests.md`:

- [x] NewReference validates non-empty memoryID
- [x] WithKey stores template correctly
- [x] Build(ctx) validates reference config
- [x] Error accumulation works across builder calls
- [x] Edge cases: empty key template, nil context handling
- [x] Build() produces valid engine memory.ReferenceConfig

## Success Criteria

- ReferenceBuilder follows context-first pattern
- All unit tests pass
- make lint and make test pass
- Reference builder ready for agent integration
