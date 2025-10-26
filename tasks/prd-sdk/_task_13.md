## status: completed

<task_context>
<domain>sdk/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 13.0: Task: Basic (S)

## Overview

Implement BasicBuilder for creating single-step agent or tool execution tasks. Most common task type for simple workflows.

<critical>
- **MANDATORY** align with engine task type: `TaskTypeBasic = "basic"`
- **MANDATORY** use context-first Build(ctx) pattern
- **MANDATORY** support both agent and tool execution modes
</critical>

<requirements>
- BasicBuilder with agent/tool execution configuration
- Input/output mapping support
- Condition support for control flow
- Final flag for workflow termination
- Error accumulation pattern
</requirements>

## Subtasks

- [x] 13.1 Create sdk/task/basic.go
- [x] 13.2 Implement BasicBuilder struct and constructor
- [x] 13.3 Add agent execution methods (WithAgent, WithAction)
- [x] 13.4 Add tool execution method (WithTool)
- [x] 13.5 Add input/output methods
- [x] 13.6 Add control flow methods (WithCondition, WithFinal)
- [x] 13.7 Implement Build(ctx) with validation
- [x] 13.8 Write unit tests

## Implementation Details

Reference: `tasks/prd-sdk/03-sdk-entities.md` (Section 5.1: Basic Task)

### Key APIs

```go
// sdk/task/basic.go
func NewBasic(id string) *BasicBuilder
func (b *BasicBuilder) WithAgent(agentID string) *BasicBuilder
func (b *BasicBuilder) WithAction(actionID string) *BasicBuilder
func (b *BasicBuilder) WithTool(toolID string) *BasicBuilder
func (b *BasicBuilder) WithInput(input map[string]string) *BasicBuilder
func (b *BasicBuilder) WithOutput(output string) *BasicBuilder
func (b *BasicBuilder) WithCondition(condition string) *BasicBuilder
func (b *BasicBuilder) WithFinal(isFinal bool) *BasicBuilder
func (b *BasicBuilder) Build(ctx context.Context) (*task.Config, error)
```

### Relevant Files

- `sdk/task/basic.go` - BasicBuilder implementation
- `engine/task/config.go` - Task config struct

### Dependent Files

- `sdk/internal/errors/build_error.go` - Error aggregation

## Deliverables

- ✅ `sdk/task/basic.go` with BasicBuilder
- ✅ Support for agent and tool execution
- ✅ Input/output mapping
- ✅ Control flow (condition, final flag)
- ✅ Build(ctx) validation
- ✅ Unit tests with table-driven cases

## Tests

Unit tests from `_tests.md`:
- [ ] Basic task with agent execution
- [ ] Basic task with tool execution
- [ ] Input/output mapping
- [ ] Condition evaluation
- [ ] Final flag configuration
- [ ] Error: both agent and tool specified (invalid)
- [ ] Error: neither agent nor tool specified
- [ ] Error: empty task ID
- [ ] BuildError aggregation

## Success Criteria

- Builder creates valid `TaskTypeBasic` config
- Agent and tool execution modes work
- Input/output mapping preserved
- Validation rejects invalid states
- Test coverage ≥95%
- `make lint && make test` pass
