---
status: pending
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

- [ ] 3.1 Implement structs and schema
- [ ] 3.2 Add validators and tests

## Sequencing

- Blocked by: 1.0
- Unblocks: 4.0, 5.0, 6.0
- Parallelizable: Yes

## Implementation Details

Use existing `engine/schema` helpers; keep plan types self‑contained under builtin/orchestrate.

### Relevant Files

- `engine/tool/builtin/definition.go`

### Dependent Files

- `engine/tool/builtin/orchestrate/handler.go`

## Success Criteria

- Plan JSON validates and maps to structs in tests
