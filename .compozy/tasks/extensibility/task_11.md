---
status: completed
title: Job, run, review, and artifact phase hook dispatches
type: backend
complexity: high
dependencies:
  - task_09
---

# Task 11: Job, run, review, and artifact phase hook dispatches

## Overview
Insert the remaining 12 hook dispatches covering job execution, run lifetime, the fix-reviews flow, and artifact writes. These include 3 job hooks, 4 run hooks, 5 review hooks, and 2 artifact hooks. Like task 10, each insertion is a thin call to `Manager.DispatchMutable` or `DispatchObserver` at the right pipeline boundary; the tests must verify that mutations land in the expected artifact or that observers see the expected payloads.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Requirements derive from `_techspec.md` and `_protocol.md` section 6.5.
</critical>

<requirements>
- MUST insert `DispatchMutable` for `job.pre_execute` before each job's execution and `DispatchObserver` for `job.post_execute` after it finishes.
- MUST insert `DispatchMutable` for `job.pre_retry` before the executor decides to retry a failed job.
- MUST insert `DispatchMutable` for `run.pre_start` once before the executor begins dispatching jobs and `DispatchObserver` for `run.post_start` immediately after.
- MUST insert `DispatchObserver` for `run.pre_shutdown` and `run.post_shutdown` around the executor's shutdown path.
- MUST insert the five review hooks (`review.pre_fetch`, `review.post_fetch`, `review.pre_batch`, `review.post_fix`, `review.pre_resolve`) in the fix-reviews command flow.
- MUST insert `artifact.pre_write` and `artifact.post_write` in the shared artifact writer path used by Host API and any other kernel writers.
- MUST honor the payload and patch shapes defined in `_protocol.md` section 6.5.
- MUST respect nil-manager no-op semantics identical to task 10.
- MUST NOT alter any job/run/review artifact on disk when the manager is nil.
</requirements>

## Subtasks
- [x] 11.1 Insert `job.pre_execute`, `job.post_execute`, and `job.pre_retry` in the executor's per-job loop in `internal/core/run/executor/execution.go`.
- [x] 11.2 Insert `run.pre_start`, `run.post_start`, `run.pre_shutdown`, and `run.post_shutdown` in the executor's top-level run flow.
- [x] 11.3 Insert the five review hooks in the fix-reviews command flow (locate the orchestration file; likely under `internal/cli` or `internal/core/run` for the fix-reviews path).
- [x] 11.4 Insert `artifact.pre_write` and `artifact.post_write` in the artifact writer path used by `host.artifacts.write` and other kernel writers.
- [x] 11.5 Add tests covering each hook with a mock extension that records or mutates payloads.

## Implementation Details
See `_protocol.md` section 6.5 for the canonical payload and patch shapes per hook. See TechSpec "Impact Analysis" rows on `internal/core/run/executor` and `internal/cli/commands.go` for the affected files.

Modified files:
- `internal/core/run/executor/execution.go` — job + run hook dispatches
- Fix-reviews orchestration file (identify during exploration; likely uses `handlers.go` or a dedicated fix-reviews package)
- `internal/core/extension/host_writes.go` — artifact hook dispatches around the artifact writer
- `internal/core/run/executor/execution_test.go` — test insertions
- `internal/core/extension/hooks_integration_test.go` — extended with review and artifact cases

Key invariants:
- `job.pre_execute` mutation replaces the job before execution starts; side effects of previous jobs cannot leak into the payload the extension sees.
- `job.pre_retry.proceed = false` cancels the retry attempt with a documented reason.
- `review.pre_resolve.resolve = false` prevents the GitHub thread resolution for that issue.
- `artifact.pre_write.cancel = true` prevents the write from happening and the caller sees an explicit error with `data.reason = "cancelled_by_extension"`.

### Relevant Files
- `internal/core/run/executor/execution.go` — Executor loop and run lifecycle.
- `internal/core/run/executor/` — Any helper files for job retry and shutdown.
- Fix-reviews orchestration file — To be identified during task execution.
- `internal/core/extension/host_writes.go` — From task 06. Artifact writer path.
- `internal/core/extension/manager.go` — From task 08.
- `_protocol.md` section 6.5 — Hook event matrix.

### Dependent Files
- Task 14 (Go SDK) relies on the payload shapes being stable after this task.
- Task 15 (TS SDK) relies on the same shapes.

### Related ADRs
- [ADR-004: Priority-Ordered Mutation Pipeline for Hooks](adrs/adr-004.md) — Dispatch semantics used here.

## Deliverables
- 12 hook dispatches inserted across job, run, review, and artifact phases.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests exercising a full `fix-reviews` run with a mock extension participating in all five review hooks **(REQUIRED)**

## Tests
- Unit tests:
  - [x] With the manager nil, the executor loop and fix-reviews flow produce exactly the same output as before this task.
  - [x] A mock extension returning `job.pre_retry.proceed = false` cancels the retry and marks the job permanently failed.
  - [x] A mock extension returning `review.pre_resolve.resolve = false` prevents the resolution thread from being posted.
  - [x] A mock extension returning `artifact.pre_write.cancel = true` prevents the file from being written and the caller receives the documented error.
  - [x] `run.post_shutdown` fires exactly once per run with the final `RunSummary`.
  - [x] `job.post_execute` fires exactly once per job with the result payload.
- Integration tests:
  - [x] End-to-end `compozy fix-reviews` against a fixture PR with a mock extension participating in all five review hooks completes successfully and the extension's recorded payloads match the issues it should have seen.
  - [x] End-to-end `compozy start` run with a mock extension chain (priority 100, 500, 900) observes all `run.*` and `job.*` hook dispatches in order.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- Existing executor and fix-reviews tests still pass without change
- All 26 hooks from `_protocol.md` section 6.5 now have at least one insertion point in the Compozy runtime (task 10 covered 14, task 11 covers the remaining 12)
