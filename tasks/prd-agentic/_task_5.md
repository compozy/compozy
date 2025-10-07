---
status: pending
parallelizable: false
blocked_by: ["1.0", "2.0", "3.0"]
---

<task_context>
<domain>engine/tool/builtin/orchestrate/executor</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>http_server</dependencies>
<unblocks>"6.0","9.0","10.0"</unblocks>
</task_context>

# Task 5.0: Implement executor (sequential + parallel) with limits

## Overview

Implement execution engine that runs plan steps using Runner. Support per‑step timeouts, concurrency caps, result merging, and bindings propagation.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- `Run(ctx, plan) ([]StepResult, error)` with errgroup for parallel.
- Enforce `max_depth`, `max_steps`, `max_parallel`; derive timeouts from parent deadline.
- Emit per‑step result structs with `exec_id`, `status`, `output`, `error`.
- Unit tests for sequencing, parallel fan‑out, and cancellations.
</requirements>

## Subtasks

- [ ] 5.1 Engine core and result model
- [ ] 5.2 Safety/limits enforcement
- [ ] 5.3 Tests (sequential, parallel, cancellations)

## Sequencing

- Blocked by: 1.0, 2.0, 3.0
- Unblocks: 6.0, 9.0, 10.0
- Parallelizable: No (core engine)

## Implementation Details

- Use `errgroup.Group` with bounded workers; prefer context propagation over globals.
- Model executor control flow with a dedicated `looplab/fsm` state machine (`pending` → `planning` → `dispatching` → `awaiting_results` → `merging` → `completed`/`failed`), following the callback + observer conventions already in `engine/llm/orchestrator`.
- Encapsulate FSM wiring in `engine/tool/builtin/orchestrate/fsm.go` so tests can assert transition tables independently from Runner integration.
- Surface transition hooks that record metrics/logs per state using `logger.FromContext(ctx)` and propagate plan/execution context through event arguments.

### Relevant Files

- `engine/agent/exec/*` (Runner)
- `engine/task/router/direct_executor.go`

### Dependent Files

- `engine/tool/builtin/orchestrate/handler.go`

## Success Criteria

- Deterministic results; limits enforced; tests pass
