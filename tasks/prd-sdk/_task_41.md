## status: completed

<task_context>
<domain>sdk/runtime</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>sdk/runtime/builder.go, engine/runtime</dependencies>
</task_context>

# Task 41.0: Runtime: Native Tools Builder (S)

## Overview

Implement native tools builder for built-in call_agents and call_workflows tools.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md
</critical>

<requirements>
- NativeToolsBuilder with call_agents and call_workflows support
- Integration with runtime builder
- Context-first Build(ctx) pattern
- Simple configuration (enable/disable features)
</requirements>

## Subtasks

- [x] 41.1 Create sdk/runtime/native_tools.go with NativeToolsBuilder struct
- [x] 41.2 Implement NewNativeTools() constructor
- [x] 41.3 Implement WithCallAgents() method
- [x] 41.4 Implement WithCallWorkflows() method
- [x] 41.5 Implement Build(ctx) returning *runtime.NativeToolsConfig
- [x] 41.6 Extend runtime Builder with WithNativeTools(tools *runtime.NativeToolsConfig) method
- [x] 41.7 Add unit tests for native tools builder

## Implementation Details

Reference from 03-sdk-entities.md section 9.2 and architecture:

```go
type NativeToolsBuilder struct {
    config *runtime.NativeToolsConfig
}

func NewNativeTools() *NativeToolsBuilder

// Enable call_agents native tool
func (b *NativeToolsBuilder) WithCallAgents() *NativeToolsBuilder

// Enable call_workflows native tool
func (b *NativeToolsBuilder) WithCallWorkflows() *NativeToolsBuilder

// Build with context (kept for consistency)
func (b *NativeToolsBuilder) Build(ctx context.Context) *runtime.NativeToolsConfig
```

Runtime builder integration:
```go
func (b *Builder) WithNativeTools(tools *runtime.NativeToolsConfig) *Builder
```

Example from architecture:
```go
runtime.NewBun().
    WithNativeTools(
        runtime.NewNativeTools().
            WithCallAgents().
            WithCallWorkflows().
            Build(ctx),
    )
```

### Relevant Files

- `sdk/runtime/native_tools.go` (new)
- `sdk/runtime/builder.go` (extend existing)
- `engine/runtime/types.go` (native tools config)

### Dependent Files

- Task 40.0 output (runtime builder base)
- Future runtime examples

## Deliverables

- `sdk/runtime/native_tools.go` implementing NativeToolsBuilder
- WithNativeTools integration in runtime Builder
- Build(ctx) producing engine runtime.NativeToolsConfig
- Package documentation

## Tests

Unit tests mapped from `_tests.md`:

- [x] NewNativeTools creates empty config
- [x] WithCallAgents enables call_agents tool
- [x] WithCallWorkflows enables call_workflows tool
- [x] Build(ctx) returns valid NativeToolsConfig
- [x] Runtime builder WithNativeTools integrates correctly
- [x] Edge cases: no tools enabled, all tools enabled

## Success Criteria

- Native tools builder follows builder pattern
- All unit tests pass
- make lint and make test pass
- Ready for runtime examples
