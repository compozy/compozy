# Task Memory: task_05.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement the daemon-owned run manager for task, review, and exec flows using the approved techspec sections "Run manager", "Data Flow", and "Run Lifecycle and Recovery".
- Preserve the existing planner/executor/run-scope/runtime semantics while moving lifecycle ownership, persistence, snapshot, watch, and cancel behavior to the daemon.
- Required evidence includes lifecycle/concurrency unit coverage, real integration coverage, and clean `make verify`.

## Important Decisions
- Use the existing kernel run-start flow and run-scope bootstrap as the execution seam; do not add a second planner/executor path.
- Keep `global.db` as the run index / conflict gate and `run.db` as the per-run event + projection store. Existing `events.jsonl`, `run.json`, and `result.json` remain compatibility outputs through the current journal/result writers.
- Keep the daemon-owned manager behind `internal/api/core.RunService`, so HTTP/UDS handlers and later CLI/TUI clients share the same lifecycle implementation.
- When the manager must synthesize a terminal event, append it with `SubmitWithSeq(...)` before reading `run.db` again; async-only journal submission can race terminal-state mirroring.

## Learnings
- `internal/api/core` already defines the daemon transport contract for runs, including dense snapshots, event pagination, SSE streaming, and cancel; task `05` needs the concrete implementation behind those interfaces.
- `journal.Open()` already opens `run.db`, mirrors events into `events.jsonl`, and serializes event persistence, so daemon lifecycle ownership should wrap that seam rather than replace it.
- Current workflow runs write `run.json` during planning and terminal events/results during execution; exec runs write their own persisted record and terminal events in `internal/core/run/exec/exec.go`.
- `model.RuntimeConfig.DaemonOwned` is now part of the daemon lifecycle contract; daemon-backed exec flows use it to suppress local-only text output behavior, and the public SDK runtime mirror must stay aligned with that field set.

## Files / Surfaces
- `internal/daemon/run_manager.go` (new manager surface)
- `internal/daemon/run_manager_test.go`
- `sdk/extension/types.go`
- `internal/api/core/{interfaces.go,handlers.go}`
- `internal/store/globaldb/{global_db.go,registry.go}`
- `internal/store/rundb/{migrations.go,run_db.go}`
- `internal/core/kernel/handlers.go`
- `internal/core/plan/prepare.go`
- `internal/core/run/executor/{execution.go,lifecycle.go,result.go}`
- `internal/core/run/exec/exec.go`
- `internal/core/model/{run_scope.go,artifacts.go,runtime_config.go}`
- `internal/core/run/journal/journal.go`

## Errors / Corrections
- Baseline correction: the concrete daemon run manager now exists in `internal/daemon/run_manager.go`; later transport/client work should build on that service instead of reintroducing lifecycle logic elsewhere.
- Verification correction: `sdk/extension.RuntimeConfig` initially drifted from `model.RuntimeConfig` after introducing `DaemonOwned`; task `05` fixed the public mirror so `sdk/extension` compatibility tests pass under `make verify`.

## Ready for Next Run
- `task_05` is complete with `go test -cover ./internal/daemon` at `81.2%` and a clean `make verify`.
- Later daemon tasks can depend on `internal/daemon.RunManager` for task/review/exec start, snapshot, event replay+live watch, and idempotent cancel semantics.
