## status: completed

<task_context>
<domain>sdk/project</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>sdk/internal/errors, sdk/internal/validate</dependencies>
</task_context>

# Task 04.0: Minimal Project Builder + Unit Test (M)

## Overview

Implement the Project builder - the top-level SDK entity that contains all resources (models, workflows, agents, etc.). Start with minimal functionality and core methods only.

<critical>
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Project Configuration section)
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Context-First Architecture)
- **MUST** implement Build(ctx context.Context) method
- **MUST** use BuildError for error accumulation
- **MUST** validate project name and version
</critical>

<requirements>
- Create ProjectBuilder with fluent API
- Implement New(name) constructor
- Implement core metadata methods (WithVersion, WithDescription, WithAuthor)
- Implement resource registration methods (AddModel, AddWorkflow, AddAgent)
- Implement Build(ctx) with validation and error accumulation
- Use BuildError for multiple validation errors
- Use validation helpers from sdk/internal/validate
</requirements>

## Subtasks

- [x] 04.1 Create sdk/project/builder.go with Builder struct
- [x] 04.2 Implement New(name) constructor
- [x] 04.3 Implement WithVersion(version) *Builder
- [x] 04.4 Implement WithDescription(desc) *Builder
- [x] 04.5 Implement WithAuthor(name, email, org) *Builder
- [x] 04.6 Implement AddModel(model) *Builder
- [x] 04.7 Implement AddWorkflow(wf) *Builder
- [x] 04.8 Implement AddAgent(agent) *Builder
- [x] 04.9 Implement Build(ctx context.Context) (*project.Config, error)
- [x] 04.10 Add comprehensive unit tests

## Implementation Details

Reference: tasks/prd-sdk/03-sdk-entities.md (Project Configuration)

### Builder Pattern

```go
// sdk/project/builder.go
package project

import (
    "context"
    "github.com/compozy/compozy/engine/project"
    "github.com/compozy/compozy/sdk/internal/errors"
    "github.com/compozy/compozy/sdk/internal/validate"
)

type Builder struct {
    config *project.Config
    errors []error
}

func New(name string) *Builder
func (b *Builder) WithVersion(version string) *Builder
func (b *Builder) WithDescription(desc string) *Builder
func (b *Builder) WithAuthor(name, email, org string) *Builder
func (b *Builder) AddModel(model *core.ProviderConfig) *Builder
func (b *Builder) AddWorkflow(wf *workflow.Config) *Builder
func (b *Builder) AddAgent(agent *agent.Config) *Builder
func (b *Builder) Build(ctx context.Context) (*project.Config, error)
```

### Relevant Files

- `sdk/project/builder.go` (NEW)
- `sdk/project/builder_test.go` (NEW)
- `engine/project/config.go` (REFERENCE)

### Dependent Files

- `sdk/internal/errors/build_error.go`
- `sdk/internal/validate/validate.go`

## Deliverables

- ✅ `sdk/project/builder.go` with complete Builder implementation
- ✅ All methods follow fluent API pattern (return *Builder)
- ✅ Build(ctx) validates and returns engine project.Config
- ✅ Error accumulation using BuildError
- ✅ Unit tests with 95%+ coverage
- ✅ Table-driven tests for validation scenarios

## Tests

Reference: tasks/prd-sdk/_tests.md

- Unit tests for Project builder:
  - [x] Test New() creates valid builder
  - [x] Test WithVersion() validates semver format
  - [x] Test WithDescription() accepts non-empty strings
  - [x] Test WithAuthor() validates email format
  - [x] Test AddModel() accumulates models
  - [x] Test AddWorkflow() accumulates workflows
  - [x] Test AddAgent() accumulates agents
  - [x] Test Build() with valid config succeeds
  - [x] Test Build() with empty name fails
  - [x] Test Build() with invalid version fails
  - [x] Test Build() with no workflows fails
  - [x] Test Build() accumulates multiple errors
  - [x] Test context propagation to logger

## Success Criteria

- Project builder follows fluent API pattern
- Build(ctx) requires context.Context
- BuildError aggregates multiple validation errors
- Validation uses helpers from sdk/internal/validate
- All tests use t.Context() instead of context.Background()
- Tests achieve 95%+ coverage
- Error messages are clear and actionable
