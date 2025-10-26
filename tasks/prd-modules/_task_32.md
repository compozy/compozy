## status: pending

<task_context>
<domain>v2/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>v2/internal/errors, engine/memory</dependencies>
</task_context>

# Task 32.0: Memory: Reference (S)

## Overview

Implement the memory reference builder (`v2/memory/reference.go`) for attaching memory to agents with key templates.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** tasks/prd-modules/02-architecture.md and tasks/prd-modules/03-sdk-entities.md
</critical>

<requirements>
- Memory reference builder with key template support
- Context-first Build(ctx) pattern
- Error accumulation following BuildError pattern
- Produces engine/memory.ReferenceConfig
</requirements>

## Subtasks

- [ ] 32.1 Create v2/memory/reference.go with ReferenceBuilder struct
- [ ] 32.2 Implement NewReference(memoryID string) constructor
- [ ] 32.3 Implement WithKey(keyTemplate string) method
- [ ] 32.4 Implement Build(ctx) with validation
- [ ] 32.5 Add unit tests for reference builder

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

- `v2/memory/reference.go` (new)
- `v2/internal/errors/build_error.go` (existing)
- `engine/memory/types.go` (engine types)

### Dependent Files

- `v2/agent/builder.go` (will use memory references)
- Future agent attachment examples

## Deliverables

- `v2/memory/reference.go` implementing ReferenceBuilder
- Constructor and fluent methods with error accumulation
- Build(ctx) producing engine memory.ReferenceConfig
- Package documentation

## Tests

Unit tests mapped from `_tests.md`:

- [ ] NewReference validates non-empty memoryID
- [ ] WithKey stores template correctly
- [ ] Build(ctx) validates reference config
- [ ] Error accumulation works across builder calls
- [ ] Edge cases: empty key template, nil context handling
- [ ] Build() produces valid engine memory.ReferenceConfig

## Success Criteria

- ReferenceBuilder follows context-first pattern
- All unit tests pass
- make lint and make test pass
- Reference builder ready for agent integration
