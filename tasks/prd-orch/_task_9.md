---
status: pending
parallelizable: true
blocked_by: ["2.0"]
---

<task_context>
<domain>engine/llm/orchestrator</domain>
<type>implementation</type>
<scope>observability</scope>
<complexity>low</complexity>
<dependencies>logging, metrics</dependencies>
<unblocks>10.0</unblocks>
</task_context>

# Task 9.0: Centralize logging/metrics per transition

## Overview

Standardize state/event labeled logging and metrics in FSM callbacks (`before_event`, `enter_*`, `after_*`) replacing scattered logs.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Use `logger.FromContext(ctx)`; include state, event, iter counters
- Add hooks for latency measurements per transition (if metrics available)
- No behavior changes; logs only
</requirements>

## Subtasks

- [ ] 9.1 Add logging hooks
- [ ] 9.2 Optional timing metrics wrappers
- [ ] 9.3 Unit tests asserting log labels presence

## Sequencing

- Blocked by: 2.0
- Unblocks: 10.0
- Parallelizable: Yes

## Implementation Details

Mirror Redis PRD’s observability discipline; ensure consistent label keys.

### Relevant Files

- `engine/llm/orchestrator/state_machine.go`
- `engine/llm/orchestrator/loop.go`

### Dependent Files

- `engine/llm/orchestrator/orchestrator.go`

## Success Criteria

- Logs carry state/event labels; lints/tests pass
