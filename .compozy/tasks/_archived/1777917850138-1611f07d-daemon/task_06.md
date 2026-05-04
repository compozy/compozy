---
status: completed
title: Reconciliation, Retention, and Graceful Shutdown
type: backend
complexity: high
dependencies:
  - task_03
  - task_05
---

# Reconciliation, Retention, and Graceful Shutdown

## Overview
This task closes the run lifecycle by defining what happens when the daemon crashes, restarts, retains old runs, or shuts down while work is still active. It makes startup reconciliation, synthetic crash events, purge behavior, and force-stop semantics explicit so operator state cannot drift after failures.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Run Lifecycle and Recovery", "Transport Contract", and "Monitoring and Observability" instead of duplicating them here
- FOCUS ON "WHAT" — recovery and shutdown semantics matter more than any single implementation detail
- MINIMIZE CODE — prefer one reconciliation path and one shutdown path instead of ad hoc fixes in multiple call sites
- TESTS REQUIRED — unit and integration coverage are mandatory for crash recovery, purge, and shutdown behavior
</critical>

<requirements>
1. MUST complete startup reconciliation before the daemon reports ready by scanning incomplete runs and marking them as `crashed`.
2. MUST append a synthetic `run.crashed` event to `run.db` when the per-run database is still present and recoverable.
3. MUST implement explicit retention and purge behavior driven by daemon configuration, including oldest-first terminal-run cleanup.
4. MUST return `409` from daemon stop while active runs exist unless `force=true` is supplied.
5. MUST cancel active runs, drain writer loops and child processes, and flush terminal state before forced shutdown exits.
</requirements>

## Subtasks
- [x] 6.1 Add daemon startup reconciliation for runs left in `starting` or `running`.
- [x] 6.2 Persist synthetic crash information consistently across `global.db` and `run.db`.
- [x] 6.3 Implement configurable retention windows and explicit run purge behavior.
- [x] 6.4 Introduce graceful and forced daemon shutdown behavior, including active-run conflicts.
- [x] 6.5 Add recovery, purge, and shutdown tests that exercise real temp databases and child-run shutdown paths.

## Implementation Details
Implement the recovery and retention model described in the TechSpec "Run Lifecycle and Recovery", "Transport Contract", and "Monitoring and Observability" sections. This task should centralize all post-crash and shutdown logic so later commands and transports only observe one consistent lifecycle contract.

### AGH Reference Files
- `~/dev/compozy/agh/internal/daemon/boot.go` — reference for startup reconciliation timing before readiness.
- `~/dev/compozy/agh/internal/daemon/daemon.go` — reference for graceful and forced shutdown behavior.
- `~/dev/compozy/agh/internal/session/manager.go` — reference for draining active sessions and runs during stop and recovery.

### Relevant Files
- `internal/daemon/reconcile.go` — new startup reconciliation logic for interrupted runs.
- `internal/daemon/shutdown.go` — new daemon stop and force-stop orchestration.
- `internal/store/globaldb/runs.go` — persisted lifecycle state and purge queries for terminal runs.
- `internal/store/rundb/run_db.go` — synthetic crash-event append logic and terminal flush behavior.
- `internal/core/run/journal/journal.go` — current append-before-publish journal semantics that must stay deterministic during crash handling.
- `internal/core/run/executor/shutdown.go` — current executor shutdown hooks that must align with daemon-owned force-stop behavior.

### Dependent Files
- `internal/api/core/handlers.go` — daemon stop, health, and purge routes depend on the conflict and recovery semantics introduced here.
- `internal/cli/state.go` — client-facing daemon status and stop commands will surface the shutdown outcomes defined here.
- `internal/core/extension/manager_shutdown.go` — extension subprocess shutdown must align with daemon force-stop and drain timing.

### Related ADRs
- [ADR-001: Adopt a Global Home-Scoped Singleton Daemon](adrs/adr-001.md) — requires robust singleton recovery after crashes and restarts.
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — requires terminal and crash state to live in operational storage rather than files.

## Deliverables
- Startup reconciliation flow for interrupted daemon-managed runs.
- Synthetic crash-event handling and mirrored terminal-state updates.
- Configurable retention and purge logic for terminal runs.
- Unit tests with 80%+ coverage for reconciliation and shutdown rules **(REQUIRED)**
- Integration tests covering crash recovery, forced stop, and purge behavior **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Reconciliation marks `starting` and `running` rows as `crashed` before readiness is reported.
  - [x] Reconciliation preserves a best-effort `error_text` summary when `run.db` cannot be reopened for synthetic crash-event append.
  - [x] Purge selects terminal runs in oldest-first order and respects configured keep-count and keep-days limits.
  - [x] Forced stop cancels active runs and preserves the final terminal state before daemon exit.
  - [x] Forced stop waits for writer loops and extension subprocess cleanup only up to the configured drain timeout before exiting.
- Integration tests:
  - [x] Restarting the daemon after a simulated crash leaves the interrupted run in `crashed` and emits a synthetic recovery event when `run.db` is still openable.
  - [x] Restarting the daemon after a simulated crash with a missing or corrupt `run.db` still marks the global run row `crashed` and keeps the daemon healthy.
  - [x] `POST /daemon/stop` returns `409` while active runs exist and succeeds when `force=true` is explicitly provided.
  - [x] Forced stop cancels a real active run, terminates child work, and leaves no run stuck in `starting` or `running`.
  - [x] `compozy runs purge` removes terminal run directories and index rows without touching active runs.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Restart recovery marks interrupted runs consistently before the daemon becomes ready
- Forced shutdown drains active work predictably and persists final state
- Retention and purge behavior are explicit, bounded, and observable
