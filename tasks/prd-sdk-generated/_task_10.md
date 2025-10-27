## status: pending

<task_context>
<domain>sdk/task</domain>
<type>implementation</type>
<scope>code_generation</scope>
<complexity>high</complexity>
<dependencies>sdk2/workflow,sdk2/tool</dependencies>
</task_context>

# Task 10.0: Migrate task Package to Functional Options

## Overview

Migrate `sdk/task` with 7+ task type variants (Basic, Parallel, Collection, Wait, Signal, Human, Conditional). Each type needs separate constructor with type-specific validation.

**Estimated Time:** 4-6 hours

**Dependencies:** Requires Tasks 6.0 (tool) and 8.0 (workflow) complete

<critical>
- **MULTIPLE VARIANTS:** 7+ separate task types with different configs
- **HIGHEST COMPLEXITY:** Most complex package in SDK migration
- **TYPE-SPECIFIC LOGIC:** Each task type has unique validation rules
</critical>

<requirements>
- Generate options for each task type separately
- Create constructors: NewBasic, NewParallel, NewCollection, NewWait, NewSignal, NewHuman, NewConditional
- Type-specific validation for each variant
- Handle task transitions (OnSuccess, OnError)
- Validate parallel task collections
- Validate conditional expressions
- Deep copy and extensive tests
</requirements>

## Subtasks

- [ ] 10.1 Create sdk2/task/ directory structure
- [ ] 10.2 Analyze task types and their engine structs
- [ ] 10.3 Create generate files for 7+ task types
- [ ] 10.4 Generate options for each type
- [ ] 10.5 Create NewBasic constructor + tests
- [ ] 10.6 Create NewParallel constructor + tests
- [ ] 10.7 Create NewCollection constructor + tests
- [ ] 10.8 Create NewWait constructor + tests
- [ ] 10.9 Create NewSignal constructor + tests
- [ ] 10.10 Create NewHuman constructor + tests
- [ ] 10.11 Create NewConditional constructor + tests
- [ ] 10.12 Integration tests across types
- [ ] 10.13 Comprehensive documentation

## Implementation Details

### Task Types & Constructors

#### 1. BasicTask
```go
func NewBasic(ctx context.Context, id string, agentID string, actionID string, opts ...BasicOption) (*task.BasicTaskConfig, error)
```
Fields: ID, AgentID, ActionID, Input, Timeout, Retry, OnSuccess, OnError

#### 2. ParallelTask
```go
func NewParallel(ctx context.Context, id string, tasks []string, opts ...ParallelOption) (*task.ParallelTaskConfig, error)
```
Fields: ID, Tasks (array of task IDs), MaxConcurrency, OnSuccess, OnError

#### 3. CollectionTask
```go
func NewCollection(ctx context.Context, id string, items []any, taskTemplate string, opts ...CollectionOption) (*task.CollectionTaskConfig, error)
```
Fields: ID, Items, TaskTemplate, Sequential, OnSuccess, OnError

#### 4. WaitTask
```go
func NewWait(ctx context.Context, id string, duration string, opts ...WaitOption) (*task.WaitTaskConfig, error)
```
Fields: ID, Duration, OnSuccess

#### 5. SignalTask
```go
func NewSignal(ctx context.Context, id string, signalName string, opts ...SignalOption) (*task.SignalTaskConfig, error)
```
Fields: ID, SignalName, Payload, WaitForResponse, Timeout

#### 6. HumanTask
```go
func NewHuman(ctx context.Context, id string, prompt string, opts ...HumanOption) (*task.HumanTaskConfig, error)
```
Fields: ID, Prompt, Inputs, Timeout, OnSuccess, OnError

#### 7. ConditionalTask
```go
func NewConditional(ctx context.Context, id string, condition string, opts ...ConditionalOption) (*task.ConditionalTaskConfig, error)
```
Fields: ID, Condition (expression), ThenTask, ElseTask

### Validation Per Type

**Basic:** Agent and action must exist
**Parallel:** At least 2 tasks, MaxConcurrency > 0
**Collection:** Items non-empty, TaskTemplate valid
**Wait:** Duration parseable
**Signal:** SignalName non-empty
**Human:** Prompt non-empty
**Conditional:** Condition syntax valid, then/else tasks defined

### Relevant Files

**Reference (for understanding):**
- `sdk/task/builder.go` - Old builder pattern to understand requirements (~300+ LOC)
- `sdk/task/builder_test.go` - Old tests to understand test cases
- `engine/task/config.go` - Source structs for all 7+ task types

**To Create in sdk2/task/:**
- `generate.go` - Code generation directives (7+ types)
- `basic_options_generated.go` - Generated options for BasicTask
- `parallel_options_generated.go` - Generated options for ParallelTask
- `collection_options_generated.go` - Generated options for CollectionTask
- `wait_options_generated.go` - Generated options for WaitTask
- `signal_options_generated.go` - Generated options for SignalTask
- `human_options_generated.go` - Generated options for HumanTask
- `conditional_options_generated.go` - Generated options for ConditionalTask
- `constructors.go` - All 7+ constructors (NewBasic, NewParallel, etc.)
- `constructors_test.go` - Extensive tests for all types
- `README.md` - Documentation for multi-type approach

**Note:** Do NOT delete or modify anything in `sdk/task/` - keep for reference during transition. All 7+ task types go in the same sdk2/task/ package.

## Tests

**Per Task Type (7×):**
- [ ] Valid minimal configuration
- [ ] Full configuration with all options
- [ ] Required fields validation
- [ ] Type-specific validation rules
- [ ] Transition configuration
- [ ] Deep copy verification

**Integration:**
- [ ] Task type mix in workflow
- [ ] Parallel tasks with subtasks
- [ ] Collection iteration patterns
- [ ] Conditional branching logic

## Success Criteria

- [ ] sdk2/task/ directory structure created
- [ ] All 7+ task types have constructors in sdk2/task/
- [ ] Type-specific validation complete
- [ ] Clear separation between task types
- [ ] Transition logic validated
- [ ] Tests pass: `gotestsum -- ./sdk2/task`
- [ ] Linter clean: `golangci-lint run ./sdk2/task/...`
- [ ] Reduction: ~300+ LOC → ~180 LOC (40% reduction)
- [ ] README documents when to use each task type
- [ ] Old sdk/task/ remains untouched
