---
status: completed
parallelizable: false
blocked_by: ["1.0", "2.0", "5.0"]
---

<task_context>
<domain>engine/llm/orchestrator</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>budgets, progress detection</dependencies>
<unblocks>7.0</unblocks>
</task_context>

# Task 6.0: Implement `update_budgets` guards (budgets, no-progress)

## Overview

Apply success/error budgets and progress detection in `enter_update_budgets`, emitting `budget_ok` or `budget_exceeded` to control loop continuation or termination.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Reuse existing budget counters and progress fingerprints
- Preserve thresholds and guard outcomes from legacy logic
- Emit structured logs for guard decisions
</requirements>

## Subtasks

- [x] 6.1 Implement guard evaluation and event emission
- [x] 6.2 Tests for boundary conditions (just under/over thresholds)

## Sequencing

- Blocked by: 1.0, 2.0, 5.0
- Unblocks: 7.0
- Parallelizable: No (depends on 5.0 metadata)

## Implementation Details

Align with Tech Spec “Guard Logic” and use `fingerprint.go` helpers.

### Relevant Files

- `engine/llm/orchestrator/loop_state.go`
- `engine/llm/orchestrator/fingerprint.go`
- `engine/llm/orchestrator/state_machine.go`

### Dependent Files

- `engine/llm/orchestrator/loop.go`

## Success Criteria

- Tests pass for guard decisions; lints/tests pass
