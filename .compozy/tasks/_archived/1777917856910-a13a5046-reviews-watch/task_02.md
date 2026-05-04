---
status: completed
title: Daemon Watch Coordinator and Push Boundary
type: backend
complexity: critical
dependencies:
  - task_01
---

# Daemon Watch Coordinator and Push Boundary

## Overview

This task implements the daemon-owned parent run that drives the review watch state machine. It owns provider waiting, review round import, child fix-run orchestration, verification, optional push, cancellation, duplicate-watch protection, and watch lifecycle events.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Data Flow", "Coordinator State Machine", "Git push", and "Watch events"
- FOCUS ON "WHAT" — preserve existing fetch/fix behavior and add only the orchestration needed around it
- MINIMIZE CODE — keep the coordinator small and isolate git operations behind a narrow boundary
- TESTS REQUIRED — cancellation, timeout, push safety, duplicate-watch rejection, and child-run failures are mandatory
</critical>

<requirements>
1. MUST create a daemon parent run mode `review_watch` that is observable through existing run storage, snapshots, streams, and cancellation.
2. MUST orchestrate existing review fetch/write and `StartReviewRun` behavior instead of creating a second remediation pipeline.
3. MUST implement the TechSpec state machine with context-bound timers or tickers, with no unowned goroutines and no `time.Sleep()` orchestration.
4. MUST prevent duplicate active watches for the same workspace, provider, and PR.
5. MUST verify zero unresolved issues and HEAD advancement for non-empty rounds before any push.
6. MUST implement a git boundary that only reads state and runs `git push <remote> HEAD:<branch>`.
7. MUST never run destructive git commands, manual staging commands, or cleanup commands.
</requirements>

## Subtasks

- [x] 2.1 Add the `review_watch` parent run mode and duplicate active-watch tracking.
- [x] 2.2 Implement the coordinator state machine over provider status, review import, child fix runs, verification, and terminal states.
- [x] 2.3 Add the narrow git state/push runner and enforce push safety invariants.
- [x] 2.4 Emit structured `review.watch_*` events with provider, PR, workflow, round, run IDs, head SHA, and errors.
- [x] 2.5 Add daemon tests for success, clean exit, max rounds, cancellation, timeout, duplicate watches, child failure, no-commit failure, and push failure.

## Implementation Details

Implement the parent-run behavior described in the TechSpec "Data Flow" and "Coordinator State Machine" sections. Review issue persistence must stay in the existing fetch writer path, and remediation must stay in the existing daemon-backed review run path.

### Relevant Files

- `internal/daemon/run_manager.go` — add parent run mode, child run orchestration entry points, and duplicate-watch integration.
- `internal/daemon/review_exec_transport_service.go` — expose daemon review watch start behavior from the review service boundary.
- `internal/daemon/run_snapshot.go` — ensure parent watch state remains visible through snapshots.
- `internal/daemon/workspace_events.go` — publish workspace-level invalidation events for watch progress.
- `pkg/compozy/events/kinds/review.go` — add typed watch lifecycle payloads.
- `pkg/compozy/events/kinds/doc.go` — document review watch event kinds for public payload compatibility.
- `internal/daemon/review_exec_transport_service_test.go` — cover coordinator behavior through daemon service tests.

### Dependent Files

- `internal/store/rundb/run_db.go` — existing run event persistence must accept the new review watch event kinds.
- `internal/daemon/run_manager_test.go` — add parent/child lifecycle and cancellation coverage.
- `internal/daemon/transport_service_test.go` — cover daemon service availability and duplicate active-watch errors.
- `.compozy/tasks/reviews-watch/_techspec.md` — source of state machine and push invariants.

### Related ADRs

- [ADR-001: Use a Daemon-Owned Parent Run for Review Watching](adrs/adr-001.md) — establishes the parent run architecture.
- [ADR-002: Require Provider Watch Status Before Declaring Reviews Clean](adrs/adr-002.md) — constrains clean terminal state.
- [ADR-003: Force Auto-Commit and Allow Dirty Worktrees for Auto-Push Watch Runs](adrs/adr-003.md) — constrains push and dirty-worktree behavior.

## Deliverables

- Daemon `review_watch` parent run mode with observable stream/snapshot state.
- Coordinator state machine for provider wait, fetch, child fix, verify, push, clean, max-rounds, failure, and cancellation.
- Git state/push runner with no destructive command surface.
- Watch lifecycle events and payload tests.
- Unit tests with 80%+ coverage for coordinator and git runner behavior **(REQUIRED)**
- Integration tests for daemon parent/child run lifecycle and cancellation **(REQUIRED)**

## Tests

- Unit tests:
  - [x] Current provider review plus zero fetched issues completes as `clean` without creating an empty review directory.
  - [x] Current provider review plus actionable issues persists the next review round and starts exactly one child review run.
  - [x] Non-empty round with successful child run but unchanged HEAD fails before push.
  - [x] `auto_push=true` runs only `git push <remote> HEAD:<branch>` after successful verification.
  - [x] Dirty worktree and existing unpushed commits emit warning metadata but do not block startup.
  - [x] Provider timeout, provider error, child failure, child cancellation, push failure, and max-rounds each produce the expected terminal state.
  - [x] Duplicate active watch for the same workspace/provider/PR returns `review_watch_already_active`.
- Integration tests:
  - [x] Fake provider and fake git runner drive a two-round daemon watch from issues to push to reviewed-clean terminal state.
  - [x] Cancelling the parent run stops provider waiting and prevents new child runs.
  - [x] Existing run snapshot and stream APIs expose parent watch events and child run references.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- `review_watch` behaves as a daemon-owned, cancellable, attachable parent run
- Push is impossible after failed verification or unchanged HEAD
- The coordinator cannot declare clean until provider status is current for the PR head
