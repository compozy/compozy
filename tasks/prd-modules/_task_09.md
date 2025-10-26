## status: pending

<task_context>
<domain>v2/tool</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>v2/internal/errors, v2/internal/validate, v2/schema</dependencies>
</task_context>

# Task 09.0: Action Builder (S)

## Overview

Implement the Tool builder for defining custom tools with input/output schemas and runtime code. Tools can execute in Bun, Node, or Deno runtimes.

<critical>
- **ALWAYS READ** tasks/prd-modules/03-sdk-entities.md (Tool Definition section)
- **MUST** validate tool ID and name
- **MUST** support input and output schemas
- **MUST** validate runtime (bun, node, deno)
</critical>

<requirements>
- Create ToolBuilder with fluent API
- Implement New(id) constructor
- Implement WithName() and WithDescription() methods
- Implement WithRuntime() method
- Implement WithCode() method
- Implement WithInput() and WithOutput() schema methods
- Implement Build(ctx) with validation
</requirements>

## Subtasks

- [ ] 09.1 Create v2/tool/builder.go with Builder struct
- [ ] 09.2 Implement New(id) constructor
- [ ] 09.3 Implement WithName(name) *Builder
- [ ] 09.4 Implement WithDescription(desc) *Builder
- [ ] 09.5 Implement WithRuntime(runtime) *Builder
- [ ] 09.6 Implement WithCode(code) *Builder
- [ ] 09.7 Implement WithInput(schema) *Builder
- [ ] 09.8 Implement WithOutput(schema) *Builder
- [ ] 09.9 Implement Build(ctx context.Context) (*tool.Config, error)
- [ ] 09.10 Add comprehensive unit tests

## Implementation Details

Reference: tasks/prd-modules/03-sdk-entities.md (Tool Definition)

### Builder Pattern

```go
// v2/tool/builder.go
package tool

import (
    "context"
    "github.com/compozy/compozy/engine/tool"
    "github.com/compozy/compozy/v2/internal/errors"
    "github.com/compozy/compozy/v2/internal/validate"
)

type Builder struct {
    config *tool.Config
    errors []error
}

func New(id string) *Builder
func (b *Builder) WithName(name string) *Builder
func (b *Builder) WithDescription(desc string) *Builder
func (b *Builder) WithRuntime(runtime string) *Builder
func (b *Builder) WithCode(code string) *Builder
func (b *Builder) WithInput(schema *schema.Schema) *Builder
func (b *Builder) WithOutput(schema *schema.Schema) *Builder
func (b *Builder) Build(ctx context.Context) (*tool.Config, error)
```

### Relevant Files

- `v2/tool/builder.go` (NEW)
- `v2/tool/builder_test.go` (NEW)
- `engine/tool/config.go` (REFERENCE)

### Dependent Files

- `v2/internal/errors/build_error.go`
- `v2/internal/validate/validate.go`
- `v2/schema/builder.go` (for schemas)

## Deliverables

- ✅ `v2/tool/builder.go` with complete Builder implementation
- ✅ Runtime validation (bun, node, deno)
- ✅ Support for input and output schemas
- ✅ Build(ctx) validates tool configuration
- ✅ Unit tests with 95%+ coverage

## Tests

Reference: tasks/prd-modules/_tests.md

- Unit tests for Tool builder:
  - [ ] Test New() creates valid builder
  - [ ] Test WithName() validates non-empty
  - [ ] Test WithDescription() accepts description
  - [ ] Test WithRuntime() validates runtime type
  - [ ] Test WithCode() validates non-empty code
  - [ ] Test WithInput() sets input schema
  - [ ] Test WithOutput() sets output schema
  - [ ] Test Build() with valid config succeeds
  - [ ] Test Build() with empty ID fails
  - [ ] Test Build() with empty name fails
  - [ ] Test Build() with invalid runtime fails
  - [ ] Test Build() with empty code fails
  - [ ] Test context propagation

## Success Criteria

- Tool builder supports all runtime types (bun, node, deno)
- Input and output schemas are properly set
- Build(ctx) requires context.Context
- Tests achieve 95%+ coverage
- Error messages are clear and actionable
