Goal (incl. success criteria):

- Implement task `task_05.md` (`Runtime Shutdown, Logging, and Storage Discipline`) in the `daemon-improvs` worktree.
- Success means daemon shutdown/close paths use bounded ownership, detached vs foreground signal/logging behavior is explicit, `internal/logger` owns daemon log sinks, `global.db` and `run.db` checkpoint before close, required tests land, and `make verify` passes cleanly.

Constraints/Assumptions:

- Work only in `/Users/pedronauck/Dev/compozy/_worktrees/daemon-improvs`.
- Must follow `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, `systematic-debugging`, `no-workarounds`, and `cy-final-verify`.
- Must read and update `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_05.md}`.
- Keep scope tight to runtime shutdown, daemon logging, and SQLite close discipline; record follow-ups instead of expanding into later liveness/reconcile tasks.
- No destructive git commands without explicit user permission.

Key decisions:

- The `daemon-improvs` worktree is the source of truth for this task; the separate `/Users/pedronauck/dev/compozy/agh` checkout is unrelated dirty state.
- Use the TechSpec sections `Component Overview`, `Technical Dependencies`, `Monitoring and Observability`, `Impact Analysis`, and `Build Order` plus ADR-002/003/004 as the implementation boundary.
- Treat the current pre-change signal as code-level gaps, not failing tests: `internal/daemon/host.go` and `internal/daemon/run_manager.go` still use unbounded `context.Background()` shutdown/close paths, `internal/store/{globaldb,rundb}` close methods only call raw `db.Close()`, `internal/logger` does not exist yet, and detached bootstrap still wires raw stdio directly to the log file from CLI code.

State:

- In progress after repository/task/ADR/memory review and pre-change gap capture.

Done:

- Read root `AGENTS.md` and `CLAUDE.md` in the `daemon-improvs` worktree.
- Read required skill instructions for `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, `testing-anti-patterns`, `systematic-debugging`, and `no-workarounds`.
- Read `.compozy/tasks/daemon-improvs/{_techspec.md,_tasks.md,task_05.md}` and ADRs `adr-002.md`, `adr-003.md`, and `adr-004.md`.
- Read workflow memory files `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_05.md}`.
- Scanned existing daemon-improvement ledgers for task-01 and task-03 cross-task context.
- Confirmed current gaps in `internal/daemon/host.go`, `internal/daemon/run_manager.go`, `internal/store/globaldb/global_db.go`, `internal/store/rundb/run_db.go`, `internal/cli/daemon_commands.go`, and the absence of `internal/logger`.

Now:

- Build the execution checklist, then patch daemon lifecycle/logging/store code and add focused unit/integration coverage.

Next:

- Update workflow memory during implementation, then update task tracking, run `make verify`, self-review, and create the scoped local commit if verification passes.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED whether the daemon logger should restore the previous global `slog.Default()` after foreground runs in tests, or keep the process-wide default until exit.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-21-MEMORY-runtime-hardening.md`
- `.compozy/tasks/daemon-improvs/{_techspec.md,_tasks.md,task_05.md}`
- `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_05.md}`
- `internal/daemon/{host.go,shutdown.go,service.go,boot.go,boot_test.go,boot_integration_test.go,run_manager.go,shutdown_test.go}`
- `internal/cli/{daemon.go,daemon_commands.go,daemon_commands_test.go,command_context.go,daemon_launch_unix.go,daemon_launch_other.go}`
- `internal/store/{sqlite.go,globaldb/global_db.go,rundb/run_db.go}`
- `cmd/compozy/main.go`
- Commands: `rg`, `sed`, `nl`, `git status --short --branch`
