---
status: completed
title: Add Tabbed Multi-Run TUI Attach Experience
type: frontend
complexity: critical
dependencies:
  - task_03
  - task_04
---

# Task 5: Add Tabbed Multi-Run TUI Attach Experience

## Overview

This task adds the tabbed TUI experience required for queued, running, completed, failed, and canceled workflows. It attaches to the daemon-owned parent run, keeps one tab per requested slug, renders the existing task-run UI inside the active tab, and preserves the current quit dialog semantics at queue level.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- The TUI MUST show one tab for every requested workflow, including queued workflows that have not started.
- The user MUST be able to navigate between tabs without closing completed child views.
- The active tab MUST render the familiar task-run UI for its child run once that child exists.
- Queued tabs MUST show clear queued/not-started state before their child run exists.
- The TUI MUST isolate child UI state by tab so job indexes, transcripts, and translators do not collide.
- `Close TUI` MUST detach from the parent and keep the queue running.
- `Stop Run` MUST cancel the parent queue, cancel the active child, and mark queued work canceled.
- `Cancel` MUST return to the multi-run TUI without changing execution.
</requirements>

## Subtasks

- [x] 5.1 Add a remote multi-run attach surface that loads parent snapshot state.
- [x] 5.2 Add tab state for ordered workflows, active tab, child run IDs, and item statuses.
- [x] 5.3 Render tabs above the existing task-run surface without changing single-run TUI layout.
- [x] 5.4 Wire tab navigation keys and help text without breaking existing pane focus controls.
- [x] 5.5 Follow parent queue events and child run streams so tabs update live.
- [x] 5.6 Map the existing quit dialog actions to parent queue detach/stop/cancel behavior.
- [x] 5.7 Add TUI model, rendering, remote attach, and quit behavior tests.

## Implementation Details

Build a wrapper around the existing remote run UI instead of rewriting the single-run cockpit. Use parent queue state from task 03 and child snapshots/streams from existing run APIs. See TechSpec "TUI" component, "Known Risks", and ADR-004 for state isolation and quit behavior.

### Relevant Files

- `internal/core/run/ui/model.go` — current single-run model and controller state.
- `internal/core/run/ui/remote.go` — current daemon-backed single-run attach flow.
- `internal/core/run/ui/view.go` — title, progress, separator, body, and help rendering.
- `internal/core/run/ui/update.go` — key handling, focus cycling, quit dialog behavior.
- `internal/core/run/ui/types.go` — UI message and state types.
- `internal/core/run/ui/remote_test.go`, `view_test.go`, `update_test.go`, and `model_test.go` — existing TUI test patterns.
- `internal/cli/run_observe.go` — CLI bridge from daemon-started runs into TUI attach sessions.

### Dependent Files

- `internal/api/client/runs.go` — existing child snapshot and stream methods remain the child observation primitive.
- `internal/api/client/client.go` — multi-run snapshot method from task 02 supplies parent state.
- `internal/daemon/run_manager.go` — parent events and cancellation semantics from task 03 drive the TUI.
- `internal/cli/daemon_commands.go` — CLI command from task 04 must attach the multi-run TUI in UI mode.

### Related ADRs

- [ADR-003: Fix V1 Command Name and Config Behavior](adrs/adr-003.md) — Requires queued workflows to appear as tabs.
- [ADR-004: Use a Daemon-Owned Sequential Multi-Run Coordinator](adrs/adr-004.md) — Defines detach, stop, and cancel behavior for the queue.

## Deliverables

- Tabbed multi-run TUI for parent run attach.
- Live tab state for queued, running, completed, failed, and canceled workflows.
- Existing quit dialog behavior mapped to parent queue actions.
- Unit tests with 80%+ coverage for multi-run TUI state and rendering **(REQUIRED)**.
- Integration-style remote attach tests for parent/child event flow **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] Initial parent snapshot with three queued items renders three tabs in order.
  - [x] Starting child `beta` changes only the `beta` tab state and does not mutate `alpha` transcript state.
  - [x] Completed tabs remain navigable after the active tab advances to the next queued workflow.
  - [x] Tab navigation changes active tab without cycling the existing job/timeline focus unexpectedly.
  - [x] `Close TUI` exits the UI without calling parent cancel.
  - [x] `Stop Run` calls parent cancel once and marks active/queued visual state as stopping or canceled.
- Integration tests:
  - [x] Remote attach reconstructs parent queue state from snapshot, then follows parent and child streams.
  - [x] Reattaching to an in-progress parent run restores completed, active, and queued tabs.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing.
- Test coverage >=80%.
- 100% of requested slugs appear as tabs before their child runs start.
- Users can navigate between queued, active, and completed tabs.
- Single-run TUI behavior remains unchanged for `tasks run`.
