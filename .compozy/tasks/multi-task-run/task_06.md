---
status: completed
title: Refactor `task_multi` Into a Mode-Aware Scheduler
type: refactor
complexity: high
dependencies:
  - task_03
  - task_04
---

# Task 6: Refactor `task_multi` Into a Mode-Aware Scheduler

## Overview

This task turns the existing sequential-only multi-run coordinator into a mode-aware scheduler. It preserves enqueued behavior while creating shared queue, event, cancellation, and terminal aggregation structure for the later parallel branch.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- The daemon MUST accept `parallel` as a valid multi-run mode.
- The scheduler MUST dispatch `enqueued` and `parallel` through explicit branches.
- The enqueued branch MUST preserve current one-child-at-a-time behavior.
- Shared helpers MUST keep event emission and snapshot state consistent across modes.
- This task MUST NOT implement worktree remapping or parallel child fanout yet.
- Existing enqueued attach and stream behavior MUST remain compatible.
</requirements>

## Subtasks

- [x] 6.1 Introduce mode-aware scheduler dispatch.
- [x] 6.2 Extract shared queue-start, item-queued, item-terminal, and queue-terminal helpers.
- [x] 6.3 Preserve current enqueued branch semantics.
- [x] 6.4 Change daemon mode validation to accept `parallel`.
- [x] 6.5 Add guarded parallel branch scaffolding for later tasks.
- [x] 6.6 Update daemon tests that currently expect parallel rejection.

## Implementation Details

Use the TechSpec "Scheduler Model" and "Scheduler Behavior" sections. Treat this as a structure-preserving refactor for enqueued mode; worktree allocation, runtime remapping, and bounded fanout are later tasks.

### Relevant Files

- `internal/daemon/task_multi.go` — current coordinator, mode validation, child wait, event emission, and snapshot builder.
- `internal/daemon/task_multi_test.go` — sequential, failure, cancellation, and preflight tests.
- `internal/daemon/run_manager.go` — dispatches `runModeTaskMulti` to the coordinator.
- `internal/daemon/service.go` — tracks daemon run mode metrics.
- `pkg/compozy/events/event.go` — declares existing multi-run event kinds.

### Dependent Files

- `internal/daemon/shutdown.go` — parent cancellation must remain coherent for active runs.
- `internal/core/run/ui/multi_remote.go` — depends on stable event ordering and statuses.
- `internal/cli/run_observe.go` — stream output depends on existing event kinds.

### Related ADRs

- [ADR-004: Use a Daemon-Owned Sequential Multi-Run Coordinator](adrs/adr-004.md) — Baseline enqueued behavior to preserve.
- [ADR-007: Use One Task-Multi Scheduler with Worktree-Owned Child Runs](adrs/adr-007.md) — Requires one scheduler with explicit mode branches.

## Deliverables

- Mode-aware `task_multi` scheduler structure.
- Preserved enqueued behavior and tests.
- Daemon validation accepting `parallel`.
- Unit tests with 80%+ coverage for scheduler dispatch and enqueued preservation **(REQUIRED)**.
- Integration tests proving enqueued snapshots remain unchanged **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] `resolveTaskMultiMode("parallel")` returns `parallel`.
  - [x] Enqueued mode starts `alpha` before `beta`.
  - [x] Enqueued mode does not start `beta` until `alpha` reaches terminal status.
  - [x] Enqueued child start failure still marks queued siblings canceled.
  - [x] Enqueued snapshots still return child items in requested order.
- Integration tests:
  - [x] Starting enqueued `alpha,beta` creates one parent and two child rows.
  - [x] Existing multi-run TUI attach tests still pass against enqueued snapshots.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- The daemon has a clean shared scheduler boundary ready for worktree-backed parallel execution.
- Current enqueued users see no regression.
