# Task Memory: task_05.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Harden the existing daemon lifecycle boundary by adding bounded shutdown ownership, explicit detached vs foreground daemon behavior, shared structured daemon logging, and checkpoint-on-close for `global.db` and `run.db`.
- Validation must include focused shutdown/logger/store tests plus the repo gate (`make verify`) before tracking or commit updates.

## Important Decisions

- Implement in `/Users/pedronauck/Dev/compozy/_worktrees/daemon-improvs`; the separate `agh` checkout is unrelated dirty state and not this task’s source of truth.
- Keep the current `Host` + `RunManager` + `global.db`/`run.db` ownership model intact and centralize shutdown/logging behavior instead of introducing new runtime layers.
- Treat the pre-change gap as explicit code debt rather than a currently failing test: host/transport close paths still use raw background contexts, DB close paths skip checkpoints, and detached bootstrap still owns log-file redirection in CLI code.

## Learnings

- The current detached daemon bootstrap already isolates itself from the parent command by launching `compozy daemon start --internal-child` with `exec.CommandContext(context.Background(), ...)`, but daemon log ownership still lives in CLI redirection instead of a shared daemon logger package.
- `RunManager` already carries `ShutdownDrainTimeout` and bounded force-stop waiting, so the missing work is mostly host/transport/close orchestration, signal policy, and database close discipline.

## Files / Surfaces

- `internal/daemon/{host.go,shutdown.go,service.go,boot.go,boot_test.go,boot_integration_test.go,run_manager.go,shutdown_test.go}`
- `internal/cli/{daemon.go,daemon_commands.go,daemon_commands_test.go,command_context.go,daemon_launch_unix.go,daemon_launch_other.go}`
- `internal/store/{sqlite.go,globaldb/global_db.go,rundb/run_db.go}`
- `cmd/compozy/main.go`

## Errors / Corrections

- Corrected task targeting early: the current shell cwd was on branch `unify-capability`, but task files, workflow memory, and prior ledgers all point to the `daemon-improvs` worktree.

## Ready for Next Run

- Pre-change signals are anchored in code: `internal/daemon/host.go` uses `context.Background()` for transport and host close paths, `internal/daemon/run_manager.go` uses `context.Background()` in `closeRunScope`, `internal/store/globaldb/global_db.go` and `internal/store/rundb/run_db.go` only call raw `db.Close()`, and `internal/logger` is still missing.
