## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/tool/builtin/orchestrate</domain>
<type>testing|documentation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 6.0: Testing and metrics for orchestrator

## Overview

Expand unit and integration tests as per `_tests.md`, and add metrics for invalid-plan, retries, parallel timing, and step outcomes.

<critical>
- **MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc
- **MUST READ THE SOURCES** before start the task
</critical>

<requirements>
- Add counters/gauges for key events; expose via monitoring infrastructure
- Achieve ≥80% coverage in planner and executor packages
- Use `t.Run("Should ...")` structure and `testify`
</requirements>

## Subtasks

- [ ] Implement metrics in orchestrate paths and register in monitoring
- [ ] Add/extend unit tests per `_tests.md`
- [ ] Add integration tests in `test/integration/`

## Implementation Details

Hook counters in handler/executor; use monitoring sink for tests where possible.

### Sources

- E2E expectations in project docs
- Industry telemetry best practices

### Relevant Files

- `engine/tool/builtin/orchestrate/planner/*_test.go`
- `engine/tool/builtin/orchestrate/executor.go`
- `engine/infra/monitoring/*`

### Dependent Files

- `test/integration/*`

## Success Criteria

- All tests pass; metrics validated; coverage targets met
