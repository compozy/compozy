## status: pending

<task_context>
<domain>v2/agent</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>v2/internal/errors, v2/internal/validate</dependencies>
</task_context>

# Task 08.0: Agent Builder (M)

## Overview

Implement the Agent builder and ActionBuilder for defining AI agents with instructions, actions, tools, knowledge, and memory. ActionBuilder is a critical component with full configuration including prompts, output schemas, tools, transitions, retry, and timeout.

<critical>
- **ALWAYS READ** tasks/prd-modules/03-sdk-entities.md (Agent Definition section)
- **MUST** implement both Agent builder and Action builder
- **MUST** support knowledge and memory bindings
- **MUST** support MCP and tool references
- **ACTION BUILDER** must support output schema, transitions, retry, timeout
</critical>

<requirements>
- Create AgentBuilder with fluent API
- Implement New(id) constructor
- Implement WithModel() and WithModelRef() methods
- Implement WithInstructions() method
- Implement WithKnowledge() and WithMemory() methods
- Implement AddAction(), AddTool(), AddMCP() methods
- Create ActionBuilder with full configuration
- Implement action prompts, output schemas, tools, transitions
</requirements>

## Subtasks

- [ ] 08.1 Create v2/agent/builder.go with AgentBuilder struct
- [ ] 08.2 Implement New(id) constructor
- [ ] 08.3 Implement WithModel(provider, model) *Builder
- [ ] 08.4 Implement WithModelRef(modelID) *Builder
- [ ] 08.5 Implement WithInstructions(instructions) *Builder
- [ ] 08.6 Implement WithKnowledge(binding) *Builder
- [ ] 08.7 Implement WithMemory(ref) *Builder
- [ ] 08.8 Implement AddAction(action) *Builder
- [ ] 08.9 Implement AddTool(toolID) *Builder
- [ ] 08.10 Implement AddMCP(mcpID) *Builder
- [ ] 08.11 Create v2/agent/action.go with ActionBuilder
- [ ] 08.12 Implement NewAction(id) constructor
- [ ] 08.13 Implement WithPrompt(prompt) for actions
- [ ] 08.14 Implement WithOutput(schema) for actions
- [ ] 08.15 Implement AddTool(toolID) for actions
- [ ] 08.16 Implement WithSuccessTransition(taskID) for actions
- [ ] 08.17 Implement WithErrorTransition(taskID) for actions
- [ ] 08.18 Implement WithRetry(maxAttempts, backoff) for actions
- [ ] 08.19 Implement WithTimeout(timeout) for actions
- [ ] 08.20 Add comprehensive unit tests for both builders

## Implementation Details

Reference: tasks/prd-modules/03-sdk-entities.md (Agent Definition)

### Builder Patterns

```go
// v2/agent/builder.go
package agent

type Builder struct {
    config *agent.Config
    errors []error
}

func New(id string) *Builder
func (b *Builder) WithModel(provider, model string) *Builder
func (b *Builder) WithInstructions(instructions string) *Builder
func (b *Builder) WithKnowledge(binding *knowledge.BindingConfig) *Builder
func (b *Builder) WithMemory(ref *memory.ReferenceConfig) *Builder
func (b *Builder) AddAction(action *agent.ActionConfig) *Builder
func (b *Builder) Build(ctx context.Context) (*agent.Config, error)

// v2/agent/action.go
type ActionBuilder struct {
    config *agent.ActionConfig
    errors []error
}

func NewAction(id string) *ActionBuilder
func (a *ActionBuilder) WithPrompt(prompt string) *ActionBuilder
func (a *ActionBuilder) WithOutput(output *schema.Schema) *ActionBuilder
func (a *ActionBuilder) WithSuccessTransition(taskID string) *ActionBuilder
func (a *ActionBuilder) WithRetry(maxAttempts int, backoff time.Duration) *ActionBuilder
func (a *ActionBuilder) WithTimeout(timeout time.Duration) *ActionBuilder
func (a *ActionBuilder) Build(ctx context.Context) (*agent.ActionConfig, error)
```

### Relevant Files

- `v2/agent/builder.go` (NEW)
- `v2/agent/action.go` (NEW)
- `v2/agent/builder_test.go` (NEW)
- `v2/agent/action_test.go` (NEW)
- `engine/agent/config.go` (REFERENCE)

### Dependent Files

- `v2/internal/errors/build_error.go`
- `v2/internal/validate/validate.go`

## Deliverables

- ✅ `v2/agent/builder.go` with complete AgentBuilder
- ✅ `v2/agent/action.go` with complete ActionBuilder
- ✅ Support for model references and inline models
- ✅ Support for knowledge and memory bindings
- ✅ Support for MCP and tool references
- ✅ Action builder with prompts, schemas, transitions, retry, timeout
- ✅ Unit tests with 95%+ coverage for both builders

## Tests

Reference: tasks/prd-modules/_tests.md

- Unit tests for Agent builder:
  - [ ] Test New() creates valid builder
  - [ ] Test WithModel() sets inline model
  - [ ] Test WithModelRef() sets model reference
  - [ ] Test WithInstructions() validates non-empty
  - [ ] Test WithKnowledge() sets knowledge binding
  - [ ] Test WithMemory() sets memory reference
  - [ ] Test AddAction() accumulates actions
  - [ ] Test AddTool() accumulates tool IDs
  - [ ] Test AddMCP() accumulates MCP IDs
  - [ ] Test Build() with valid config succeeds
  - [ ] Test Build() with empty ID fails
  - [ ] Test Build() with no instructions fails

- Unit tests for Action builder:
  - [ ] Test NewAction() creates valid builder
  - [ ] Test WithPrompt() validates non-empty
  - [ ] Test WithOutput() sets output schema
  - [ ] Test AddTool() for action-specific tools
  - [ ] Test WithSuccessTransition() sets success path
  - [ ] Test WithErrorTransition() sets error path
  - [ ] Test WithRetry() validates positive attempts
  - [ ] Test WithTimeout() validates positive duration
  - [ ] Test Build() with valid config succeeds

## Success Criteria

- Agent builder supports model references and inline models
- Agent builder supports knowledge and memory bindings
- Action builder supports prompts, schemas, transitions
- Action builder supports retry and timeout configuration
- Build(ctx) requires context.Context for both builders
- Tests achieve 95%+ coverage
- Error messages are clear and actionable
