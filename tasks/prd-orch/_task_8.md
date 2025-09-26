---
status: completed
parallelizable: false
blocked_by: ["1.0", "2.0", "7.0"]
---

<task_context>
<domain>engine/llm/orchestrator</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>memory</dependencies>
<unblocks>9.0, 10.0</unblocks>
</task_context>

# Task 8.0: Implement `finalize` (memory persist, output packaging)

## Overview

Persist memories and package final `core.Output` in `enter_finalize`, returning it to callers via the FSM-driven loop; ensure compatibility with existing memory behavior.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Call `memory.StoreAsync` as before
- Package output in FSM metadata for `Run` to return
- Maintain logging and error propagation
</requirements>

## Subtasks

- [x] 8.1 Implement finalize actions and return value plumbing
- [x] 8.2 Unit tests ensuring output/memory semantics unchanged

## Sequencing

- Blocked by: 1.0, 2.0, 7.0
- Unblocks: 9.0, 10.0
- Parallelizable: No

## Implementation Details

Follow Tech Spec finalize sequence; confirm no changes to public behavior.

### Relevant Files

- `engine/llm/orchestrator/memory.go`
- `engine/llm/orchestrator/state_machine.go`

### Dependent Files

- `engine/llm/orchestrator/loop.go`

## Success Criteria

- Final output returned exactly as before; lints/tests pass
