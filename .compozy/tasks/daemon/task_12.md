---
status: completed
title: TUI Remote Attach and Watch
type: frontend
complexity: high
dependencies:
  - task_04
  - task_05
  - task_11
---

# TUI Remote Attach and Watch

## Overview
This task adapts the Bubble Tea UI from in-process execution to remote attach over daemon snapshots and event streams. It preserves the current interactive experience while making reattach, watch-only, and detached execution consistent with the daemon control plane.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "CLI/TUI clients", "Runs", and "SSE Contract" instead of duplicating them here
- FOCUS ON "WHAT" — preserve the current operator experience while changing the source of truth from local process memory to daemon snapshots and streams
- MINIMIZE CODE — adapt the existing UI model and update loop instead of redesigning the TUI
- TESTS REQUIRED — unit and integration coverage are mandatory for snapshot boot, stream updates, and reconnect behavior
</critical>

<requirements>
1. MUST hydrate the TUI from `GET /runs/:run_id/snapshot` and continue from the returned cursor over the daemon event stream.
2. MUST preserve the current interaction model for navigation, resizing, summary rendering, and timeline following.
3. MUST handle stream shutdown, reconnect, idle heartbeat, and remote EOF cases without corrupting UI state.
4. MUST support explicit attach behavior for an already-running daemon-managed run and keep watch-only behavior separate from full TUI attach.
5. SHOULD keep the current visual contract stable so existing users do not perceive daemonization as a UI regression.
</requirements>

## Subtasks
- [x] 12.1 Adapt the TUI boot path to load dense run snapshots before any live stream subscription starts.
- [x] 12.2 Replace local event sourcing with daemon stream consumption and cursor tracking.
- [x] 12.3 Preserve layout, navigation, summary, and timeline behavior under remote attach semantics.
- [x] 12.4 Add reconnect and shutdown handling for attach and watch flows.
- [x] 12.5 Add UI and integration tests covering snapshot restore, stream updates, and attach behavior.

## Implementation Details
Implement the remote UI contract described in the TechSpec "CLI/TUI clients", "Runs", and "SSE Contract" sections. This task should keep the current Bubble Tea experience recognizable while moving all runtime state sourcing to daemon snapshots and streams.

### AGH Reference Files
- `~/dev/compozy/agh/internal/api/core/sse.go` — reference for resume, heartbeat, and overflow stream behavior.
- `~/dev/compozy/agh/internal/observe/observer.go` — reference for dense snapshot and observation query patterns that feed remote clients.

### Relevant Files
- `internal/core/run/ui/model.go` — current TUI state model that must load from daemon snapshots instead of local execution state.
- `internal/core/run/ui/update.go` — current event handling and focus updates that must adapt to remote stream events.
- `internal/core/run/ui/view.go` — rendering logic that should remain visually stable under the new attach model.
- `internal/core/run/ui/summary.go` — summary rendering that depends on dense snapshot state.
- `internal/core/run/ui/layout.go` — resize and panel layout behavior that must remain stable during remote attach.
- `internal/core/run/ui/sidebar.go` — navigation state that depends on correct snapshot hydration.

### Dependent Files
- `internal/cli/run.go` — attach and watch command behavior must bootstrap the TUI from daemon state rather than local execution.
- `internal/cli/root_command_execution_test.go` — interactive CLI tests need to cover the new attach/watch bootstrap path.
- `pkg/compozy/runs/watch.go` — watch behavior must align with the same stream semantics the TUI now consumes.
- `internal/api/core/sse.go` — heartbeat, overflow, and resume behavior directly affect remote UI correctness.

### Related ADRs
- [ADR-003: Expose the Daemon Through AGH-Aligned REST Transports Using Gin](adrs/adr-003.md) — provides the snapshot and stream contract the TUI must consume.
- [ADR-004: Preserve TUI-First UX While Introducing Auto-Start and Explicit Workspace Operations](adrs/adr-004.md) — requires TUI-first behavior to survive the daemon migration.

## Deliverables
- TUI bootstrap from daemon snapshots and stream cursors.
- Remote attach behavior for active and persisted runs.
- Stable watch and reconnect handling for daemon-backed run observation.
- Unit tests with 80%+ coverage for model/update behavior **(REQUIRED)**
- Integration tests covering snapshot restore, attach, and stream disconnect handling **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Loading a run snapshot hydrates the expected job tree, transcript window, and navigation state before live events arrive.
  - [x] Stream events update the active job, timeline, and summary views without regressing resize or focus behavior.
  - [x] Overflow or reconnect-required frames preserve the last acknowledged cursor and do not corrupt UI state.
  - [x] Heartbeat and EOF frames do not corrupt UI state and keep reconnect logic deterministic.
  - [x] Attaching to a completed run snapshot renders the final state without requiring a live stream to stay open.
- Integration tests:
  - [x] `compozy runs attach <run_id>` restores a running daemon-managed run from snapshot and continues consuming subsequent events.
  - [x] A disconnected TUI client can reconnect using the last known cursor and continue rendering the same run.
  - [x] Attaching to a completed run renders the final snapshot cleanly and exits without waiting for new stream traffic.
  - [x] Watch-only mode streams events without launching the full TUI and keeps the attach path separate.
  - [x] A stream interruption during an active run resumes without duplicating already rendered events in the timeline.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The TUI can attach to daemon-managed runs without relying on in-process execution state
- Existing navigation and visual behavior remain stable under remote attach
- Reconnect and watch behavior are explicit and reliable
