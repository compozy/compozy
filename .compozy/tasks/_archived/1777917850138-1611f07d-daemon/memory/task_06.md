# Task Memory: task_06.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Complete daemon recovery, retention, purge, and shutdown semantics so interrupted runs reconcile before readiness, forced stop is explicit, and terminal cleanup is bounded and observable.

## Important Decisions
- Added one reconciliation path in `internal/daemon/reconcile.go` that marks `starting`/`running` rows as `crashed` before readiness and appends `run.crashed` only when the existing `run.db` is still openable.
- Centralized stop/purge behavior in `internal/daemon/shutdown.go` plus `internal/daemon/service.go` instead of scattering force-stop checks across handlers or transports.
- Put retention and drain controls in home-scoped `[runs]` config with defaults (`keep_terminal_days=14`, `keep_max=200`, `shutdown_drain_timeout=30s`).
- Added a minimal `compozy runs purge` command so terminal cleanup is actually reachable from the operator surface required by the techspec.

## Learnings
- The reconciliation path must reject missing or malformed `run.db` files up front; otherwise the normal SQLite open helper can recreate or recover files that should stay in best-effort mode.
- Simplifying `globaldb.Register` from a deferred read-then-write transaction to insert-or-ignore plus readback removes the SQLite lock conflict, but test ID generators must also be concurrency-safe under `-race`.
- Covering the daemon service surface (`Status`, `Health`, `Metrics`) was the smallest route to clear the task’s daemon-package coverage floor.

## Files / Surfaces
- `internal/daemon/{reconcile.go,reconcile_test.go,shutdown.go,shutdown_test.go,service.go,service_test.go,run_manager.go,run_manager_test.go}`
- `internal/store/globaldb/{runs.go,runs_test.go,registry.go,migrations_test.go}`
- `internal/store/rundb/{run_db.go,run_db_test.go}`
- `internal/core/workspace/{config.go,config_merge.go,config_types.go,config_validate.go}`
- `internal/core/run/journal/journal.go`
- `internal/cli/{runs.go,root.go,root_test.go,root_command_execution_test.go}`
- `pkg/compozy/events/{event.go,event_test.go,docs_test.go,kinds/run.go}`
- `docs/events.md`

## Errors / Corrections
- Fixed daemon compile regressions from the `closeRunScope` signature change and missing `globaldb` import in the new shutdown path.
- Tightened reconciliation after tests showed missing/corrupt `run.db` files were being treated as appendable instead of best-effort failures.
- Fixed repository-wide verification fallout:
  - removed the SQLite register lock by changing workspace registration to insert-or-ignore plus durable readback
  - made the globaldb test `newID` helper atomic to satisfy `-race`
  - addressed lint issues around copied range values, no-context requests, and terminal-status constants

## Ready for Next Run
- Task complete after `make verify`.
- Follow-on daemon client work should wire real daemon stop/status callers onto `internal/daemon.Service` rather than reimplementing stop conflicts or retention selection.
