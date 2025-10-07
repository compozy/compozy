---
status: pending
parallelizable: false
blocked_by: ["2.0","3.0","4.0","5.0"]
---

<task_context>
<domain>engine/tool/builtin/orchestrate</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
<unblocks>"7.0","10.0","11.0"</unblocks>
</task_context>

# Task 6.0: Implement cp__agent_orchestrate builtin handler

## Overview

Create builtin definition with input/output schemas, handler glue to planner/executor, and telemetry hooks.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Definition ID `cp__agent_orchestrate`; register input/output schemas.
- Parse input, compile (prompt → plan) if needed, execute plan, return structured result.
- Record metrics via existing builtin telemetry helpers.
</requirements>

## Subtasks

- [ ] 6.1 Definition and schema wiring
- [ ] 6.2 Handler implementation
- [ ] 6.3 Tests for happy path and failure cases

## Sequencing

- Blocked by: 2.0, 3.0, 4.0, 5.0
- Unblocks: 7.0, 10.0, 11.0
- Parallelizable: No

## Implementation Details

Follow existing cp__ tools patterns (fetch/exec/filesystem) for error/metrics.

### Relevant Files

- `engine/tool/builtin/*`

### Dependent Files

- `engine/tool/native/catalog.go`

## Success Criteria

- Tool is discoverable and callable; returns step results
