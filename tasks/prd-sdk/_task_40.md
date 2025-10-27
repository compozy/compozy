## status: completed

<task_context>
<domain>sdk/runtime</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>sdk/internal/errors, engine/runtime</dependencies>
</task_context>

# Task 40.0: Runtime: Bun (Base) (S)

## Overview

Implement the runtime builder base with Bun runtime support and core configuration.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md
</critical>

<requirements>
- Runtime builder with Bun support
- Entrypoint configuration
- Bun-specific permissions
- Memory limits
- Context-first Build(ctx) pattern
- Error accumulation following BuildError pattern
</requirements>

## Subtasks

- [x] 40.1 Create sdk/runtime/builder.go with Builder struct
- [x] 40.2 Implement NewBun() constructor
- [x] 40.3 Implement WithEntrypoint(path string) method
- [x] 40.4 Implement WithBunPermissions(permissions ...string) method
- [x] 40.5 Implement WithMaxMemoryMB(mb int) method
- [x] 40.6 Implement Build(ctx) with validation
- [x] 40.7 Add unit tests for runtime builder

## Implementation Details

Reference from 03-sdk-entities.md section 9.1:

```go
type Builder struct {
    config *runtime.Config
    errors []error
}

// Constructors for different runtime types
func NewBun() *Builder

// Entrypoint
func (b *Builder) WithEntrypoint(path string) *Builder

// Bun-specific permissions
func (b *Builder) WithBunPermissions(permissions ...string) *Builder

// Memory limits
func (b *Builder) WithMaxMemoryMB(mb int) *Builder

func (b *Builder) Build(ctx context.Context) (*runtime.Config, error)
```

Bun permissions examples:
- "--allow-read"
- "--allow-env"
- "--allow-write"

Example from architecture:
```go
runtime.NewBun().
    WithEntrypoint("./tools/main.ts").
    WithBunPermissions("--allow-read", "--allow-env").
    WithMaxMemoryMB(512)
```

### Relevant Files

- `sdk/runtime/builder.go` (new)
- `sdk/internal/errors/build_error.go` (existing)
- `engine/runtime/types.go` (engine types)

### Dependent Files

- Task 41.0 (native tools builder)
- Future runtime examples

## Deliverables

- `sdk/runtime/builder.go` implementing Runtime Builder
- NewBun() constructor and Bun configuration methods
- Build(ctx) producing engine runtime.Config
- Package documentation

## Tests

Unit tests mapped from `_tests.md`:

- [x] NewBun creates Bun runtime config
- [x] WithEntrypoint sets entrypoint path
- [x] WithBunPermissions stores permissions correctly
- [x] WithMaxMemoryMB sets memory limit
- [x] Build(ctx) validates entrypoint is required
- [x] Build(ctx) validates max memory > 0
- [x] Error cases: empty entrypoint, invalid permissions
- [x] Edge cases: multiple permission flags, relative paths

## Success Criteria

- Runtime builder follows context-first pattern
- All unit tests pass
- make lint and make test pass
- Ready for native tools integration
