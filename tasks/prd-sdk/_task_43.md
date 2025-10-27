## status: completed

<task_context>
<domain>sdk/runtime</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>engine/runtime</dependencies>
</task_context>

# Task 43.0: Runtime: Permissions + Tool Timeouts (S)

## Overview

Complete the runtime builder with permissions configuration (Bun-specific) and tool-level timeouts. This task finalizes runtime configuration surface area beyond the core features already implemented in tasks 40-42.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md
- **NEVER** use global config singleton - MUST use `config.FromContext(ctx)`
- **NEVER** use `context.Background()` in tests - MUST use `t.Context()`
</critical>

<requirements>
- Complete permission methods for all runtime types (bun)
- Add tool timeout configuration
- Validate permission strings based on runtime type
- All Build() methods require context.Context
- Follow error accumulation pattern (BuildError)
</requirements>

## Subtasks

- [x] 43.1 Add WithBunPermissions() method with validation
- [x] 43.4 Add WithToolTimeout() method (per-tool timeout configuration)
- [x] 43.5 Add validation for permission strings per runtime type
- [x] 43.6 Write unit tests for permission builders
- [x] 43.7 Write integration test with engine runtime config

## Implementation Details

Per 03-sdk-entities.md section 9.1:

```go
// sdk/runtime/builder.go
func (b *Builder) WithBunPermissions(permissions ...string) *Builder
func (b *Builder) WithToolTimeout(timeout time.Duration) *Builder
```

### Relevant Files

- `sdk/runtime/builder.go` - Runtime builder (extend)
- `engine/runtime/config.go` - Engine runtime types

### Dependent Files

- `sdk/project/builder.go` - Project uses runtime config
- `engine/runtime/executor.go` - Consumes runtime config

## Deliverables

- [x] Runtime builder methods for permissions (all 3 runtime types)
- [x] Tool timeout configuration method
- [x] Validation for permission strings per runtime type
- [x] Unit tests covering all permission methods
- [x] Integration test verifying engine config compatibility

## Tests

From _tests.md:

- Unit tests:
  - [x] WithBunPermissions() validates Bun-specific permission strings
  - [x] WithToolTimeout() validates timeout duration (positive)
  - [x] Build() fails with invalid permission combinations
  - [x] Error accumulation pattern for multiple invalid permissions

- Integration tests:
  - [x] SDK runtime config matches engine runtime.Config structure
  - [x] Permissions propagate correctly to engine executor

## Success Criteria

- All permission builder methods implemented and validated
- Tool timeout configuration working
- Unit tests passing with 100% coverage for new methods
- Integration test confirms engine compatibility
- `make lint && make test` passes
