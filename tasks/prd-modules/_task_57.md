## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>v2/*</domain>
<type>testing</type>
<scope>unit_tests</scope>
<complexity>medium</complexity>
<dependencies>task_56,task_01-52</dependencies>
</task_context>

# Task 57.0: Unit Tests: Builders (M)

## Overview

Create comprehensive unit tests for all SDK builder packages, achieving 100% code coverage with table-driven tests covering success paths, validation errors, and edge cases.

<critical>
- **ALWAYS READ** tasks/prd-modules/07-testing-strategy.md before starting
- **ALWAYS READ** tasks/prd-modules/_tests.md
- **ALWAYS READ** .cursor/rules/test-standards.mdc
- **TARGET:** 100% code coverage for all builders
- **MUST** use t.Context() (NEVER context.Background)
- **MUST** use testutil helpers from Task 56.0
</critical>

<requirements>
- Test all 30 builders across 16 categories
- Table-driven tests for all builders
- Cover success paths, validation errors, edge cases
- Test Build(ctx) context propagation
- Test error aggregation (BuildError)
- Achieve 100% coverage (verified with go test -cover)
</requirements>

## Subtasks

- [ ] 57.1 Unit tests: Project builder (validation, resource registration)
- [ ] 57.2 Unit tests: Model builder (parameters, validation ranges)
- [ ] 57.3 Unit tests: Workflow builder (agents, tasks, schedules)
- [ ] 57.4 Unit tests: Agent builder + ActionBuilder (instructions, tools, memory)
- [ ] 57.5 Unit tests: All 9 task builders (basic, parallel, collection, router, wait, aggregate, composite, signal, memory)
- [ ] 57.6 Unit tests: Knowledge builders (embedder, vectordb, source, base, binding)
- [ ] 57.7 Unit tests: Memory builders (config, reference, full features)
- [ ] 57.8 Unit tests: MCP builder (transports, headers, proto, sessions)
- [ ] 57.9 Unit tests: Runtime builder + NativeToolsBuilder
- [ ] 57.10 Unit tests: Tool, Schema, Schedule, Monitoring builders
- [ ] 57.11 Unit tests: Compozy lifecycle builder
- [ ] 57.12 Unit tests: Client builder
- [ ] 57.13 Verify 100% coverage across all packages

## Implementation Details

**Based on:** tasks/prd-modules/07-testing-strategy.md, tasks/prd-modules/_tests.md

### Test Structure Pattern

```go
// v2/workflow/builder_test.go
package workflow

import (
    "testing"

    "github.com/compozy/compozy/v2/internal/testutil"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestWorkflowBuilder_Success(t *testing.T) {
    ctx := testutil.NewTestContext(t)  // Uses t.Context()

    agent := testutil.NewTestAgent("assistant")
    task := testutil.NewTestTask("t1")

    wf, err := New("test-workflow").
        WithDescription("Test workflow").
        AddAgent(agent).
        AddTask(task).
        Build(ctx)

    require.NoError(t, err)
    assert.Equal(t, "test-workflow", wf.ID)
    assert.Len(t, wf.Agents, 1)
    assert.Len(t, wf.Tasks, 1)
}

func TestWorkflowBuilder_Validation(t *testing.T) {
    ctx := testutil.NewTestContext(t)

    tests := []testutil.TableTest{
        {
            Name: "empty ID",
            BuildFunc: func(ctx context.Context) (interface{}, error) {
                return New("").Build(ctx)
            },
            WantErr: true,
            ErrContains: "ID is required",
        },
        {
            Name: "no tasks",
            BuildFunc: func(ctx context.Context) (interface{}, error) {
                return New("test").Build(ctx)
            },
            WantErr: true,
            ErrContains: "at least one task",
        },
        // ... more validation cases
    }

    testutil.RunTableTests(t, tests)
}

func TestWorkflowBuilder_EdgeCases(t *testing.T) {
    ctx := testutil.NewTestContext(t)

    // Very long ID
    longID := strings.Repeat("a", 1000)
    _, err := New(longID).Build(ctx)
    require.Error(t, err)

    // Special characters
    _, err = New("test@#$%").Build(ctx)
    require.Error(t, err)

    // Duplicate agent IDs
    agent := testutil.NewTestAgent("agent1")
    _, err = New("test").
        AddAgent(agent).
        AddAgent(agent).  // Duplicate
        Build(ctx)
    require.Error(t, err)
}
```

### Coverage Targets by Package

- v2/project: 100% (critical path)
- v2/model: 100%
- v2/workflow: 100%
- v2/agent: 100%
- v2/task/*: 100% (all 9 types)
- v2/knowledge/*: 100% (all 5 builders)
- v2/memory: 100%
- v2/mcp: 100%
- v2/runtime: 100%
- v2/tool: 100%
- v2/schema: 100%
- v2/schedule: 100%
- v2/monitoring: 100%
- v2/compozy: 100%
- v2/client: 100%

### Relevant Files

- All builder packages: v2/*/builder.go
- Test utilities: v2/internal/testutil/
- Testing strategy: tasks/prd-modules/07-testing-strategy.md
- Test plan: tasks/prd-modules/_tests.md

### Dependent Files

- Task 56.0 deliverable (testutil package)
- All builder implementations (Tasks 1-52)

## Deliverables

- Unit tests for all 30 builders with 100% coverage:
  - `v2/project/builder_test.go`
  - `v2/model/builder_test.go`
  - `v2/workflow/builder_test.go`
  - `v2/agent/builder_test.go`
  - `v2/task/*/builder_test.go` (9 files)
  - `v2/knowledge/*/builder_test.go` (5 files)
  - `v2/memory/*/builder_test.go` (2 files)
  - `v2/mcp/builder_test.go`
  - `v2/runtime/builder_test.go`
  - `v2/tool/builder_test.go`
  - `v2/schema/builder_test.go`
  - `v2/schedule/builder_test.go`
  - `v2/monitoring/builder_test.go`
  - `v2/compozy/builder_test.go`
  - `v2/client/builder_test.go`
- Each test file must include:
  - Success path tests
  - Validation error tests (table-driven)
  - Edge case tests
  - Context propagation tests

## Tests

Coverage verification:
- [ ] Run `go test -cover ./v2/...` and verify 100% coverage
- [ ] Run `go test -coverprofile=coverage.out ./v2/...`
- [ ] Run `go tool cover -func=coverage.out | grep -v "100.0%"` (should be empty)
- [ ] All tests pass with `go test -race ./v2/...`

Quality checks:
- [ ] All tests use testutil.NewTestContext (no context.Background)
- [ ] All validation errors are tested
- [ ] BuildError accumulation is tested in multi-error scenarios
- [ ] Edge cases include: empty values, nil pointers, duplicates, long strings
- [ ] No test skips or TODOs

Compliance:
- [ ] `make lint` passes for all test files
- [ ] No golangci-lint warnings
- [ ] Tests follow .cursor/rules/test-standards.mdc

## Success Criteria

- 100% code coverage across all v2/ packages (verified by CI)
- All validation rules have corresponding tests
- Edge cases are comprehensively covered
- Tests are maintainable with table-driven approach
- No use of context.Background() in any test
- BuildError testing validates error accumulation
- Test execution time < 10 seconds for all unit tests
- Zero test flakiness (10 consecutive runs pass)
