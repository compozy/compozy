---
status: completed
parallelizable: true
blocked_by: ["1.0"]
---

<task_context>
<domain>engine/tool/builtin/orchestrate</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies></dependencies>
<unblocks>"4.0","5.0","6.0"</unblocks>
</task_context>

# Task 3.0: Define orchestration plan model and schema

## Overview

Define Go structs and JSON Schema for `Plan`, `AgentStep`, and `ParallelStep` with validation rules and template‑ready `with` maps.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Create `plan.go` and `schema.go` with JSON Schema used by builtin input validation.
- Support `result_key` naming for downstream bindings.
- Tests for schema validation and decode errors.
</requirements>

## Subtasks

- [x] 3.1 Implement structs and schema
- [x] 3.2 Add validators and tests

## Sequencing

- Blocked by: 1.0
- Unblocks: 4.0, 5.0, 6.0
- Parallelizable: Yes

## Implementation Details

- Use existing `engine/schema` helpers; keep plan types self-contained under builtin/orchestrate.
- Define canonical step identifiers and status enums that the orchestrator state machine will consume; mirror the snake-case naming used in `engine/llm/orchestrator`.
- Capture per-step transition metadata (e.g., allowed next events, failure branch identifiers) in the schema so the executor's FSM built with `github.com/looplab/fsm` can deterministically advance.

### Relevant Files

- `engine/tool/builtin/definition.go`

### Dependent Files

- `engine/tool/builtin/orchestrate/handler.go`

## Success Criteria

- Plan JSON validates and maps to structs in tests

## Progress

- Implemented orchestration plan domain types with validation helpers covering IDs, statuses, transition references, and result key uniqueness (`engine/tool/builtin/orchestrate/plan.go`).
- Authored dedicated JSON Schema with mutual exclusivity for agent/parallel payloads and forward-compatible top-level properties (`engine/tool/builtin/orchestrate/schema.go`).
- Expanded unit coverage (including schema validation) and benchmark to exercise decoding paths (`engine/tool/builtin/orchestrate/plan_test.go`).
- Eliminated redundant map→struct double marshaling by adopting `mapstructure` decoding for plan payloads.

## Validation

- `go test ./engine/tool/builtin/orchestrate`
- `make lint`
- `make test`
