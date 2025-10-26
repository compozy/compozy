## status: pending

<task_context>
<domain>v2/workflow</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>v2/internal/errors, v2/internal/validate</dependencies>
</task_context>

# Task 05.0: Minimal Workflow Builder + Unit Test (M)

## Overview

Implement the Workflow builder for constructing workflows with agents and tasks. Start with core functionality: ID, description, agents, tasks, and basic input/output configuration.

<critical>
- **ALWAYS READ** tasks/prd-modules/03-sdk-entities.md (Workflow Construction section)
- **ALWAYS READ** tasks/prd-modules/02-architecture.md (Context-First Architecture)
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

- [ ] 05.1 Create v2/workflow/builder.go with Builder struct
- [ ] 05.2 Implement New(id) constructor
- [ ] 05.3 Implement WithDescription(desc) *Builder
- [ ] 05.4 Implement AddAgent(agent) *Builder
- [ ] 05.5 Implement AddTask(task) *Builder
- [ ] 05.6 Implement WithInput(schema) *Builder
- [ ] 05.7 Implement WithOutputs(outputs) *Builder
- [ ] 05.8 Implement Build(ctx context.Context) (*workflow.Config, error)
- [ ] 05.9 Add comprehensive unit tests

## Implementation Details

Reference: tasks/prd-modules/03-sdk-entities.md (Workflow Construction)

### Builder Pattern

```go
// v2/workflow/builder.go
package workflow

import (
    "context"
    "github.com/compozy/compozy/engine/workflow"
    "github.com/compozy/compozy/v2/internal/errors"
    "github.com/compozy/compozy/v2/internal/validate"
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

- `v2/workflow/builder.go` (NEW)
- `v2/workflow/builder_test.go` (NEW)
- `engine/workflow/config.go` (REFERENCE)

### Dependent Files

- `v2/internal/errors/build_error.go`
- `v2/internal/validate/validate.go`

## Deliverables

- ✅ `v2/workflow/builder.go` with complete Builder implementation
- ✅ All methods follow fluent API pattern
- ✅ Build(ctx) validates and returns engine workflow.Config
- ✅ Error accumulation using BuildError
- ✅ Unit tests with 95%+ coverage
- ✅ Table-driven tests for validation scenarios

## Tests

Reference: tasks/prd-modules/_tests.md

- Unit tests for Workflow builder:
  - [ ] Test New() creates valid builder
  - [ ] Test WithDescription() accepts non-empty strings
  - [ ] Test AddAgent() accumulates agents
  - [ ] Test AddTask() accumulates tasks
  - [ ] Test WithInput() sets input schema
  - [ ] Test WithOutputs() sets output mapping
  - [ ] Test Build() with valid config succeeds
  - [ ] Test Build() with empty ID fails
  - [ ] Test Build() with no tasks fails
  - [ ] Test Build() with duplicate task IDs fails
  - [ ] Test Build() accumulates multiple errors
  - [ ] Test context propagation to logger

## Success Criteria

- Workflow builder follows fluent API pattern
- Build(ctx) requires context.Context
- BuildError aggregates multiple validation errors
- Validation uses helpers from v2/internal/validate
- All tests use t.Context() instead of context.Background()
- Tests achieve 95%+ coverage
- Error messages are clear and actionable
