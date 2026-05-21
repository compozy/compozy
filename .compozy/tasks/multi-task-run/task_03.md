---
status: completed
title: Implement Daemon-Owned Sequential Multi-Run Coordinator
type: backend
complexity: critical
dependencies:
  - task_01
  - task_02
---

# Task 3: Implement Daemon-Owned Sequential Multi-Run Coordinator

## Overview

This task implements the daemon-owned parent queue that makes `Close TUI` keep the whole multi-run running in the background. The coordinator preflights all slugs, starts one normal task child at a time, links child runs to the parent, emits parent lifecycle state, and stops on the first child failure.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- The implementation MUST add a parent daemon run mode for multi-task orchestration, such as `task_multi`.
- The implementation MUST preflight every requested slug before creating the parent run.
- The implementation MUST start child runs sequentially in the requested order.
- Each child task run MUST remain a normal `task` run and MUST set `ParentRunID` to the parent run ID.
- The coordinator MUST stop on the first failed child run and leave later queued items not started.
- Parent cancellation MUST cancel the active child and mark queued items canceled.
- The parent stream MUST expose enough append-only queue state for later TUI attach and reattach.
- The implementation MUST NOT add parallel execution or worktree orchestration in V1.
</requirements>

## Subtasks

- [x] 3.1 Add parent run mode and active-run accounting for multi-run parents.
- [x] 3.2 Add preflight logic that validates every slug before parent run creation.
- [x] 3.3 Add coordinator state for ordered queued/running/completed/failed/canceled items.
- [x] 3.4 Start child task runs through the existing task-run machinery with parent linkage.
- [x] 3.5 Emit parent queue lifecycle events and reconstructable snapshot state.
- [x] 3.6 Implement cancellation behavior for active and queued children.
- [x] 3.7 Add run manager tests for success, child failure, duplicate/invalid slugs, and cancellation.

## Implementation Details

Model this after the existing review-watch parent/child run pattern, but keep task children as regular `task` runs. See TechSpec "System Architecture", "Data Models", and "Monitoring and Observability" for the parent mode, child linkage, queue states, and event requirements.

### Relevant Files

- `internal/daemon/run_manager.go` — owns daemon run modes, active runs, parent/child linkage, cancellation, and execution dispatch.
- `internal/daemon/review_watch.go` — existing daemon-owned parent/child coordinator pattern.
- `internal/daemon/task_transport_service.go` — service bridge from API handlers to the run manager.
- `internal/store/globaldb/registry.go` — durable run row shape already includes `ParentRunID`.
- `pkg/compozy/events/event.go` — event kind registry for parent queue lifecycle events.
- `pkg/compozy/events/kinds/task.go` or a new event payload file — typed payloads for multi-run queue events.
- `internal/daemon/run_manager_test.go` and `internal/daemon/review_watch_test.go` — relevant test patterns.

### Dependent Files

- `internal/api/core/handlers.go` — calls the service method added in task 02.
- `internal/api/client/client.go` — later CLI task depends on the run manager behavior behind this client surface.
- `internal/core/run/ui/remote.go` — later TUI task consumes parent snapshots and child run IDs.
- `internal/daemon/service.go` and `internal/daemon/shutdown.go` — metrics and active-run counts must include the new parent mode.

### Related ADRs

- [ADR-001: Use Multi-Task Execution as Explicit Task-Run Orchestration](adrs/adr-001.md) — Treats each workflow as an independent child run.
- [ADR-004: Use a Daemon-Owned Sequential Multi-Run Coordinator](adrs/adr-004.md) — Defines daemon ownership, stop-on-failure, child linkage, and queue-level quit behavior.

## Deliverables

- Daemon-owned sequential parent coordinator for multi-run task queues.
- Child task runs linked with `ParentRunID`.
- Parent queue events and snapshot reconstruction state.
- Cancellation behavior that cancels active child work and queued items.
- Unit tests with 80%+ coverage for coordinator state transitions **(REQUIRED)**.
- Run manager integration tests for parent/child lifecycle and global DB linkage **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] Starting `alpha,beta` creates one `task_multi` parent and later child task runs in order.
  - [x] Child runs have `ParentRunID` equal to the parent run ID.
  - [x] A failed first child marks the parent failed and does not start the second child.
  - [x] A successful first child starts the second child only after the first reaches a terminal state.
  - [x] Parent cancellation cancels the active child and marks queued items canceled.
  - [x] Invalid or completed workflow preflight prevents parent run creation.
- Integration tests:
  - [x] Run manager list/snapshot APIs expose the parent and child rows with correct modes and relationships.
  - [x] Parent event log can reconstruct queued, running, completed, failed, and canceled item state.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing.
- Test coverage >=80%.
- Multi-run queues continue under daemon ownership after client detach.
- No V1 code path starts multiple child task runs concurrently.
- Existing `StartTaskRun` behavior and tests remain stable.
