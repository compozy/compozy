## status: completed

<task_context>
<domain>sdk/workflow</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>sdk/internal/errors, sdk/internal/validate</dependencies>
</task_context>

# Task 05.0: Minimal Workflow Builder + Unit Test (M)

## Overview

Implement the Workflow builder for constructing workflows with agents and tasks. Start with core functionality: ID, description, agents, tasks, and basic input/output configuration.

<critical>
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Workflow Construction section)
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Context-First Architecture)
- **MUST** implement Build(ctx context.Context) method
- **MUST** use BuildError for error accumulation
- **MUST** validate workflow ID and at least one task
</critical>

<requirements>
- Create WorkflowBuilder with fluent API
- Implement New(id) constructor
- Implement WithDescription(desc) method
- Implement AddAgent(agent) method
- Implement AddTask(task) method
- Implement WithInput(schema) and WithOutputs(outputs) methods
- Implement Build(ctx) with validation
- Use BuildError for error accumulation
</requirements>

## Subtasks

- [x] 05.1 Create sdk/workflow/builder.go with Builder struct
- [x] 05.2 Implement New(id) constructor
- [x] 05.3 Implement WithDescription(desc) *Builder
- [x] 05.4 Implement AddAgent(agent) *Builder
- [x] 05.5 Implement AddTask(task) *Builder
- [x] 05.6 Implement WithInput(schema) *Builder
- [x] 05.7 Implement WithOutputs(outputs) *Builder
- [x] 05.8 Implement Build(ctx context.Context) (*workflow.Config, error)
- [x] 05.9 Add comprehensive unit tests

## Implementation Details

Reference: tasks/prd-sdk/03-sdk-entities.md (Workflow Construction)

### Builder Pattern

```go
// sdk/workflow/builder.go
package workflow

import (
    "context"
    "github.com/compozy/compozy/engine/workflow"
    "github.com/compozy/compozy/sdk/internal/errors"
    "github.com/compozy/compozy/sdk/internal/validate"
)

type Builder struct {
    config *workflow.Config
    errors []error
}

func New(id string) *Builder
func (b *Builder) WithDescription(desc string) *Builder
func (b *Builder) AddAgent(agent *agent.Config) *Builder
func (b *Builder) AddTask(task *task.Config) *Builder
func (b *Builder) WithInput(schema *schema.Schema) *Builder
func (b *Builder) WithOutputs(outputs map[string]string) *Builder
func (b *Builder) Build(ctx context.Context) (*workflow.Config, error)
```

### Relevant Files

- `sdk/workflow/builder.go` (NEW)
- `sdk/workflow/builder_test.go` (NEW)
- `engine/workflow/config.go` (REFERENCE)

### Dependent Files

- `sdk/internal/errors/build_error.go`
- `sdk/internal/validate/validate.go`

## Deliverables

- ✅ `sdk/workflow/builder.go` with complete Builder implementation
- ✅ All methods follow fluent API pattern
- ✅ Build(ctx) validates and returns engine workflow.Config
- ✅ Error accumulation using BuildError
- ✅ Unit tests with 95%+ coverage
- ✅ Table-driven tests for validation scenarios

## Tests

Reference: tasks/prd-sdk/_tests.md

- Unit tests for Workflow builder:
  - [x] Test New() creates valid builder
  - [x] Test WithDescription() accepts non-empty strings
  - [x] Test AddAgent() accumulates agents
  - [x] Test AddTask() accumulates tasks
  - [x] Test WithInput() sets input schema
  - [x] Test WithOutputs() sets output mapping
  - [x] Test Build() with valid config succeeds
  - [x] Test Build() with empty ID fails
  - [x] Test Build() with no tasks fails
  - [x] Test Build() with duplicate task IDs fails
  - [x] Test Build() accumulates multiple errors
  - [x] Test context propagation to logger

## Success Criteria

- Workflow builder follows fluent API pattern
- Build(ctx) requires context.Context
- BuildError aggregates multiple validation errors
- Validation uses helpers from sdk/internal/validate
- All tests use t.Context() instead of context.Background()
- Tests achieve 95%+ coverage
- Error messages are clear and actionable
