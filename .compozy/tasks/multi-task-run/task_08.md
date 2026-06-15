---
status: completed
title: Implement Bounded Parallel Fanout and Fail-Late Aggregation
type: backend
complexity: critical
dependencies:
  - task_07
---

# Task 8: Implement Bounded Parallel Fanout and Fail-Late Aggregation

## Overview

This task implements true parallel multi-run scheduling. It starts remapped child runs concurrently up to the resolved limit, owns all child goroutines through the parent lifecycle, continues siblings after failures, and computes the aggregate parent result after all active children settle.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- Parallel mode MUST bound active child starts by the resolved parallel limit.
- Every child worker goroutine MUST be owned by the parent coordinator and joined before parent terminal status.
- Child allocation, start, runtime, or terminal failure MUST NOT cancel siblings in parallel mode.
- Parent status MUST be `failed` when any child fails, crashes, or cannot start.
- Parent status MUST be `completed` only when every child completes.
- Parent cancellation MUST cancel running children and mark not-started children canceled.
- Cancellation and failure events MUST be idempotent and reconstructable.
</requirements>

## Subtasks

- [x] 8.1 Add a context-aware fanout limiter for parallel child starts.
- [x] 8.2 Start child workers with explicit parent-owned lifecycle and join semantics.
- [x] 8.3 Collect child outcomes without stopping siblings on failure.
- [x] 8.4 Compute aggregate parent status and error summary.
- [x] 8.5 Fan out parent cancellation to running and queued children.
- [x] 8.6 Add race-safe daemon tests for concurrency, failure continuation, and cancellation.

## Implementation Details

Use the TechSpec "Parallel Branch" and "Terminal And Cancellation Semantics" sections. Existing first-failure behavior should remain specific to enqueued mode; parallel mode is fail-late.

### Relevant Files

- `internal/daemon/task_multi.go` — central scheduler, child wait, cancellation, and event emission code.
- `internal/daemon/task_multi_test.go` — scheduler, failure, cancellation, and snapshot tests.
- `internal/daemon/run_manager.go` — active run tracking and cancellation behavior.
- `internal/daemon/shutdown.go` — daemon stop semantics for active runs.
- `internal/store/globaldb/runs.go` — terminal child status reads.

### Dependent Files

- `internal/core/run/ui/multi_remote.go` — depends on reconstructable event ordering.
- `internal/cli/run_observe.go` — depends on parent terminal status and event stream output.
- `internal/daemon/service.go` — metrics should continue counting active and terminal run modes.

### Related ADRs

- [ADR-005: Ship Worktree-Backed Parallel Multi-Run as an Opt-In Mode](adrs/adr-005.md) — Requires bounded fanout and fail-late aggregation.
- [ADR-007: Use One Task-Multi Scheduler with Worktree-Owned Child Runs](adrs/adr-007.md) — Requires shared scheduler semantics.
- [ADR-008: Make the Parallel Multi-Run Limit Configurable](adrs/adr-008.md) — Defines the resolved limit used by the semaphore.

## Deliverables

- Parallel scheduler branch with bounded fanout.
- Fail-late aggregate parent result.
- Parent cancellation fanout to active children and queued items.
- Race-safe concurrency tests.
- Unit tests with 80%+ coverage for aggregation helpers **(REQUIRED)**.
- Integration tests for daemon-owned parallel parent/child execution **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] Parallel scheduler with limit `2` never has more than two active children.
  - [x] Failure in `alpha` does not prevent `beta` from starting.
  - [x] Parent error summary names failed child slugs after all children settle.
  - [x] Parent cancellation marks not-started items canceled.
  - [x] Repeated cancellation does not emit conflicting item states.
- Integration tests:
  - [x] Parallel parent with all successful children finishes completed.
  - [x] Parallel parent with mixed success/failure finishes failed after all children settle.
  - [x] Canceling a parallel parent cancels all running child rows.
  - [x] `go test -race ./internal/daemon` passes for the new scheduler tests.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Parallel mode runs child task workflows concurrently within the configured limit.
- Parent result deterministically reflects all child outcomes.
