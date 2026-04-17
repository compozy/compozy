---
status: pending
title: Daemon Run Manager
type: backend
complexity: critical
dependencies:
  - task_02
  - task_03
  - task_04
---

# Daemon Run Manager

## Overview
This task wraps Compozy's existing planner, executor, run scope, and extension runtime inside a daemon-owned run manager instead of reimplementing execution from scratch. It becomes the operational core for task runs, review runs, and exec runs, and it defines the snapshot, cancel, and concurrency behavior that all clients depend on.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Run manager", "Data Flow", and "Run Lifecycle and Recovery" instead of duplicating them here
- FOCUS ON "WHAT" — preserve the current run engine behavior while moving lifecycle ownership to the daemon
- MINIMIZE CODE — adapt the existing execution seam rather than creating a second planner or executor stack
- TESTS REQUIRED — unit and integration coverage are mandatory for lifecycle, cancellation, and concurrency behavior
</critical>

<requirements>
1. MUST reuse the existing planner, executor, `RunScope`, journal, and extension runtime instead of creating a separate execution implementation for daemon mode.
2. MUST create the home-scoped run directory and `run.db`, then insert the `global.db.runs` row transactionally before any child process or agent runtime starts.
3. MUST support daemon-owned start, snapshot, attach-state, watch, and cancel operations for task runs, review runs, and exec runs.
4. MUST allow concurrent runs with different `run_id` values while rejecting duplicate `run_id` creation with conflict semantics.
5. MUST mirror terminal run state into both `global.db` and `run.db` before the daemon reports the run as finished.
</requirements>

## Subtasks
- [ ] 5.1 Introduce the daemon-owned run manager contract and wire it to the shared transport layer.
- [ ] 5.2 Adapt the current planner and executor to start under daemon lifecycle ownership without changing workflow semantics.
- [ ] 5.3 Persist initial run allocation, mode, presentation mode, and cancellation state before child work begins.
- [ ] 5.4 Expose dense run snapshots and cancellation semantics for task, review, and exec flows.
- [ ] 5.5 Add concurrency and lifecycle tests for duplicate `run_id`, parallel runs, cancellation, and terminal-state mirroring.

## Implementation Details
Implement the run orchestration layer described in the TechSpec "Run manager", "Data Flow", and "Run Lifecycle and Recovery" sections. This task should make the daemon the owner of run lifecycle and persistence while keeping the current execution engine, extension hooks, and event semantics intact.

### AGH Reference Files
- `~/dev/compozy/agh/internal/session/manager.go` — reference for manager-style ownership of active runs and sessions.
- `~/dev/compozy/agh/internal/daemon/daemon.go` — reference for integrating run/session management into daemon lifecycle.
- `~/dev/compozy/agh/internal/store/sessiondb/session_db.go` — reference for coupling active lifecycle to per-run durable state.

### Relevant Files
- `internal/core/model/run_scope.go` — current run scope bootstrap and the main seam for daemon-owned runtime allocation.
- `internal/core/run/executor/execution.go` — existing workflow execution entrypoint that should remain the execution engine.
- `internal/core/run/executor/lifecycle.go` — current lifecycle state transitions that must align with daemon-owned status tracking.
- `internal/core/run/exec/exec.go` — current ad hoc exec flow that must be routed through the same daemon manager.
- `internal/core/extension/runtime.go` — current per-run extension runtime that must remain bound to the daemon-owned run scope.
- `internal/daemon/run_manager.go` — new daemon orchestration layer for task, review, and exec runs.

### Dependent Files
- `internal/api/core/handlers.go` — transport handlers will call the run manager for create, snapshot, watch, and cancel behavior.
- `internal/store/globaldb/runs.go` — run index persistence and conflict enforcement will depend on the lifecycle contract defined here.
- `internal/store/rundb/run_db.go` — per-run persistence must align with the run-manager creation and completion flow.
- `internal/core/run/ui/model.go` — later remote attach behavior depends on the dense snapshot shape emitted by this task.

### Related ADRs
- [ADR-001: Adopt a Global Home-Scoped Singleton Daemon](adrs/adr-001.md) — makes the daemon the owner of active run lifecycle.
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — requires run lifecycle to be persisted through `global.db` and `run.db`.
- [ADR-004: Preserve TUI-First UX While Introducing Auto-Start and Explicit Workspace Operations](adrs/adr-004.md) — depends on stable attach and cancel semantics from daemon-owned runs.

## Deliverables
- Daemon-owned run manager for task, review, and exec flows.
- Transactional run creation integrating `global.db`, `run.db`, and the existing execution engine.
- Dense snapshot and cancellation semantics shared by CLI, TUI, and future web clients.
- Unit tests with 80%+ coverage for lifecycle transitions and conflict handling **(REQUIRED)**
- Integration tests covering real run startup, parallel runs, and cancellation behavior **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Starting a run allocates `run.db`, records the `runs` row, and rejects duplicate `run_id` creation with a conflict error.
  - [ ] Cancelling a running task run transitions the run into the expected terminal state and persists the final status in both stores.
  - [ ] Cancelling the same run twice remains idempotent and does not corrupt the persisted lifecycle state.
  - [ ] Creating task, review, and exec runs produces mode-specific snapshots without changing the shared lifecycle contract.
  - [ ] A start failure after run allocation but before child execution rolls back or marks persisted state consistently instead of leaving an orphaned `running` row.
- Integration tests:
  - [ ] Two runs with different `run_id` values can execute concurrently in the same workspace without corrupting each other's persisted state.
  - [ ] Starting a second run with the same explicit `run_id` returns `409` even when requested concurrently from two clients.
  - [ ] A daemon-backed task run emits a dense snapshot that includes job tree, transcript window, and next cursor for later attach flows.
  - [ ] A cancelled daemon-backed run mirrors the same terminal state to both `global.db` and `run.db` before clients observe completion.
  - [ ] An exec run started through the daemon persists `mode=exec` and follows the same cancel and completion semantics as workflow runs.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon owns run lifecycle without replacing the current execution engine
- Duplicate `run_id` creation is blocked while distinct runs can execute concurrently
- Task, review, and exec flows share one persisted lifecycle and snapshot contract
