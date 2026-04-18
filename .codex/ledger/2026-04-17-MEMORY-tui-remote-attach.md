Goal (incl. success criteria):

- Implement daemon task `12` (`TUI Remote Attach and Watch`) so daemon-backed task runs can restore the existing Bubble Tea cockpit from `GET /runs/:run_id/snapshot`, continue from the returned cursor over SSE, support explicit `runs attach` / `runs watch`, preserve cockpit layout/navigation/summary behavior, add reconnect handling, and finish with clean `make verify`.

Constraints/Assumptions:

- Must follow `AGENTS.md`, `CLAUDE.md`, `.compozy/tasks/daemon/task_12.md`, `.compozy/tasks/daemon/_techspec.md`, `.compozy/tasks/daemon/_tasks.md`, ADR-003, ADR-004, and workflow memory under `.compozy/tasks/daemon/memory/`.
- Required skills in use: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`; `cy-final-verify` is mandatory before any completion claim or commit.
- `brainstorming` is intentionally skipped because task `12` already has an approved techspec/ADR-backed design and the user asked for direct implementation.
- Worktree is already dirty in unrelated daemon tracking and ledger files; do not touch or revert unrelated changes.
- No destructive git commands without explicit user permission.

Key decisions:

- Keep the existing cockpit view/update code and adapt the data source by converting daemon snapshot + stream state back into the existing `uiMsg` flow instead of redesigning the UI.
- Use the daemon snapshot as the required boot source, but enrich that snapshot enough to restore stable per-job metadata and summary state before live stream subscription begins.
- Keep watch-only and full TUI attach as separate CLI flows built on the same daemon stream contract.

State:

- Completed after clean verification.

Done:

- Read repository instructions, required skill docs, daemon workflow memory, task `12`, `_tasks.md`, `_techspec.md`, ADR-003, ADR-004, and relevant daemon ledgers from tasks `05`, `08`, and `11`.
- Confirmed current baseline:
  - daemon already exposes `/api/runs/:run_id/snapshot`, `/events`, and `/stream`
  - `tasks run` resolves attach mode but only starts the run and prints a summary line
  - `runs` currently only exposes `purge`
  - the TUI still hydrates only from in-process event bus messages
  - `pkg/compozy/runs/watch.go` still follows workspace files instead of daemon streams.
- Identified the key snapshot gap: current `RunSnapshot` is too thin for a faithful cockpit restore because persisted `job_state.summary_json` is overwritten by later lifecycle events and loses queued metadata needed by the UI.
- Added dense daemon snapshot reconstruction in `internal/daemon/run_snapshot.go` and expanded `apicore.RunSnapshot` / `RunJobState` so remote clients can restore job metadata, usage, session snapshots, and shutdown state before live events arrive.
- Added daemon run observation client support in `internal/api/client/runs.go` and refactored CLI observation so `compozy tasks run`, `compozy runs attach`, and `compozy runs watch` all use daemon snapshot + stream semantics.
- Adapted the Bubble Tea cockpit to hydrate from daemon snapshots and resume over SSE through `internal/core/run/ui/remote.go`, including heartbeat, overflow, EOF, and reconnect handling while preserving existing layout/navigation behavior.
- Added daemon-backed watch helpers in `pkg/compozy/runs/remote_watch.go` plus unit/integration coverage across CLI/UI/watch behavior.
- Verified focused coverage above the task threshold (`internal/core/run/ui` 81.2%, `pkg/compozy/runs` 81.1%) and finished with passing `make verify`.
- Created local commit `0a8c0c9` (`feat: add daemon-backed run attach and watch`) and reran `make verify` against the committed tree successfully.

Now:

- None.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-tui-remote-attach.md`
- `.compozy/tasks/daemon/{task_12.md,_tasks.md,_techspec.md}`
- `.compozy/tasks/daemon/memory/{MEMORY.md,task_12.md}`
- `.compozy/tasks/daemon/adrs/{adr-003.md,adr-004.md}`
- `internal/api/{client,core}`
- `internal/daemon/run_manager.go`
- `internal/cli/{daemon_commands.go,runs.go,root_command_execution_test.go,daemon_commands_test.go}`
- `internal/core/run/ui/*`
- `pkg/compozy/runs/watch.go`
- `internal/daemon/run_snapshot.go`
- `internal/cli/run_observe.go`
- `pkg/compozy/runs/remote_watch.go`
- Commands: `rg`, `sed -n`, `go test ./internal/cli ./internal/core/run/ui ./pkg/compozy/runs ./internal/daemon -count=1`, `go test -cover ./internal/core/run/ui ./pkg/compozy/runs`, `make verify`
