Goal (incl. success criteria):

- Complete extensibility task 09 by enabling executable extension bootstrap for `start` and `fix-reviews`, adding opt-in `--extensions` for `exec`, and ensuring the run scope starts before planning and shuts down on normal/error/cancel teardown.
- Success requires the task deliverables and tests, clean `make verify`, updated workflow memory/tracking, and one local commit after verification.

Constraints/Assumptions:

- Follow repository `AGENTS.md` / `CLAUDE.md`, task 09, `_techspec.md`, `_tasks.md`, and ADR-002.
- Required skills in use: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, `cy-final-verify`.
- Keep scope tight to command/bootstrap integration; do not add later hook insertion work from tasks 10-11.
- Do not touch unrelated dirty task files from earlier tasks.

Key decisions:

- Carry the per-invocation extension toggle through runtime config instead of baking it into a static root dispatcher dependency, so one dispatcher can support `start=true`, `fix-reviews=true`, and `exec` opt-in.
- Start the manager in kernel handling after `OpenRunScope` returns, not inside `OpenRunScope`, to preserve task 07/08 runtime construction behavior and tests.
- Scoped `exec` runs will reuse the opened run artifacts and journal when extensions are enabled so the manager, audit log, and runtime events share one run scope.

State:

- Implemented and verified; tracking updates and the required local commit remain.

Done:

- Read repository instructions, required skill files, shared workflow memory, current task memory, task 09 spec, `_techspec.md`, `_tasks.md`, and ADR-002.
- Scanned prior extension ledgers for task 02, 07, and 08 handoff details.
- Inspected `internal/cli`, `internal/core/kernel`, `internal/core/model`, `internal/core/extension`, and exec/runtime tests to identify the current bootstrap gap.
- Captured the pre-change signal: the root dispatcher has only static `OpenRunScopeOptions`, `exec` bypasses run-scope allocation entirely, and `runStartHandler` does not start the manager before planning or preserve cancellation into teardown.
- Added the `exec --extensions` flag and per-command runtime toggle plumbing so `start` / `fix-reviews` always enable executable extensions while `exec` remains opt-in.
- Refactored kernel run-start handling to preserve the fast exec path, start the manager before planning, pass scoped exec runs the same run scope, and preserve cancellation into teardown.
- Added CLI/kernel unit tests, CLI integration coverage for `exec` disabled vs enabled and `start`, plus a cancellation integration test that confirms active extensions receive shutdown on run cancellation.
- Recorded lifecycle audit entries for `initialize` and `shutdown` to make activation/shutdown assertions observable in real spawn tests.
- Cleared the remaining lint blockers (`gocyclo`, `noctx`, and staticcheck nil-context warnings) and reran fresh package tests plus `make verify` successfully.

Now:

- Update task tracking and create the required code-only local commit.

Next:

- Remove this ledger after completion if cleanup is desired in a follow-up.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-10-MEMORY-extension-bootstrap.md`
- `.compozy/tasks/extensibility/task_09.md`
- `.compozy/tasks/extensibility/_tasks.md`
- `.compozy/tasks/extensibility/_techspec.md`
- `.compozy/tasks/extensibility/adrs/adr-002.md`
- `.compozy/tasks/extensibility/memory/MEMORY.md`
- `.compozy/tasks/extensibility/memory/task_09.md`
- `internal/cli/commands.go`
- `internal/cli/commands_test.go`
- `internal/cli/state.go`
- `internal/cli/root_test.go`
- `internal/cli/root_command_execution_test.go`
- `internal/cli/testdata/exec_help.golden`
- `internal/core/api.go`
- `internal/core/kernel/commands/run_start.go`
- `internal/core/kernel/commands/commands_test.go`
- `internal/core/kernel/handlers_extensions_test.go`
- `internal/core/kernel/run_scope_cancellation_integration_test.go`
- `internal/core/kernel/handlers.go`
- `internal/core/kernel/deps_test.go`
- `internal/core/extension/manager.go`
- `internal/core/extension/manager_spawn.go`
- `internal/core/extension/manager_shutdown.go`
- `internal/core/model/runtime_config.go`
- `internal/core/model/run_scope.go`
- `internal/core/run/run.go`
- `internal/core/run/exec/exec.go`
- `internal/core/run/exec/exec_integration_test.go`
- `internal/core/run/run_test.go`
- Commands: `rg`, `sed`, `git status --short`, `go test ./internal/cli ./internal/core/kernel ./internal/core/extension`, `go test -cover ./internal/cli ./internal/core/kernel`, `make verify`
