---
status: completed
title: Render Parallel Worktree Handoff in TUI and CLI Output
type: frontend
complexity: high
dependencies:
  - task_04
  - task_08
---

# Task 9: Render Parallel Worktree Handoff in TUI and CLI Output

## Overview

This task makes parallel multi-run output actionable by surfacing worktree metadata wherever users observe a parent run. It updates the tabbed TUI, stream output, and final command handoff so every child shows final status, run id, and preserved worktree path.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- The multi-run TUI MUST render worktree path and preservation status from initial snapshots.
- The multi-run TUI MUST apply later worktree metadata events to existing tabs.
- Stream output MUST include worktree metadata for relevant child events.
- Final non-TUI output MUST include slug, final status, child run id, and worktree path for every requested child.
- Missing worktree metadata MUST render as empty or unknown for backward compatibility.
- Aggregate failure, cancellation, and crashed parent statuses MUST still return non-zero command exit behavior.
</requirements>

## Subtasks

- [x] 9.1 Extend multi-run tab state to hold worktree metadata.
- [x] 9.2 Render worktree metadata in the selected child detail or status area.
- [x] 9.3 Apply metadata from parent events after initial snapshot load.
- [x] 9.4 Update stream formatting for multi-run child events.
- [x] 9.5 Add final handoff output from the parent snapshot.
- [x] 9.6 Add TUI and CLI output tests.

## Implementation Details

Use the TechSpec "UI And Output" section. Keep the existing tabbed attach surface and child transcript model; do not add a new dashboard.

### Relevant Files

- `internal/core/run/ui/multi_remote.go` — tab state, snapshot load, parent event handling, and child stream wiring.
- `internal/core/run/ui/multi_remote_test.go` — multi-run attach and rendering tests.
- `internal/cli/run_observe.go` — stream event formatting and terminal watch behavior.
- `internal/cli/daemon_commands.go` — start handling and non-UI multi-run output.
- `internal/cli/daemon_commands_test.go` — CLI stream and output assertions.

### Dependent Files

- `internal/api/contract/types.go` — supplies snapshot item metadata.
- `pkg/compozy/events/kinds/task.go` — supplies parent event metadata.
- `internal/daemon/task_multi.go` — produces terminal snapshot state.

### Related ADRs

- [ADR-005: Ship Worktree-Backed Parallel Multi-Run as an Opt-In Mode](adrs/adr-005.md) — Requires status and worktree path visibility.
- [ADR-006: Use a Speed-First Parallel MVP for the PRD Scope](adrs/adr-006.md) — Defines clear final handoff as part of the MVP.
- [ADR-007: Use One Task-Multi Scheduler with Worktree-Owned Child Runs](adrs/adr-007.md) — Makes snapshot/TUI/stream the canonical child view.

## Deliverables

- TUI rendering for worktree path and preservation status.
- Stream output for worktree metadata on multi-run events.
- Final handoff summary with slug, status, run id, and path.
- Backward-compatible rendering when metadata is absent.
- Unit tests with 80%+ coverage for render/update helpers **(REQUIRED)**.
- Integration tests for CLI stream and final handoff output **(REQUIRED)**.

## Tests

- Unit tests:
  - [x] Initial snapshot with worktree path renders that path for the selected child.
  - [x] Parent child-started event with metadata updates an existing tab.
  - [x] Snapshot without metadata renders unknown or empty path without panic.
  - [x] Stream formatter includes worktree path for child-started and child-terminal events.
  - [x] Final summary formats children in requested order.
- Integration tests:
  - [x] `tasks run --multiple alpha,beta --parallel --stream` prints final status and worktree paths.
  - [x] Mixed child result prints aggregate failure and each child path.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Users can find every preserved worktree from the TUI, stream output, and final summary.
- Existing enqueued snapshots still render correctly.
