Goal (incl. success criteria):

- Implement task 11: convert the CLI foundation from direct execution to a daemon client for daemon-backed flows.
- Success means client-side daemon bootstrap/handshake exists, workspace and presentation mode resolve before request dispatch, legacy `compozy start` is removed, required tests pass, and `make verify` passes.

Constraints/Assumptions:

- Must follow task 11, `_techspec.md`, `_tasks.md`, ADR-001, ADR-003, ADR-004, workflow memory, `AGENTS.md`, and `CLAUDE.md`.
- Must keep scope tight; later tasks own fuller migration of some command families.
- Must not use destructive git commands.
- Must update workflow memory before finishing and promote only durable context to shared workflow memory.
- Completion requires `cy-final-verify` discipline and `make verify`.

Key decisions:

- Treat task 11 as the daemon-client foundation plus `tasks run` migration and command-surface cleanup, while leaving fuller `workspaces`/`reviews`/`exec` command completion to later tasks `14` and `15`.
- Preserve the existing start-task config plumbing under `commandKindStart`, but remove the root `compozy start` entrypoint so there is no second direct-execution CLI surface.

State:

- Completed after clean verification.

Done:

- Read governing instructions, required skill docs, workflow memory, task docs, tech spec, ADRs, and adjacent task docs.
- Audited current CLI, workspace config, daemon bootstrap, transport, and relevant tests.
- Added the daemon client transport package plus daemon host runtime wiring for transport-backed task execution.
- Added `internal/cli/daemon_commands.go` with shared bootstrap, attach-mode resolution, and daemon-backed `tasks run` dispatch.
- Extended workspace/global config precedence with `runs.default_attach_mode`.
- Removed the root `compozy start` command entrypoint and updated root/task help coverage plus execution tests.
- Verified focused task-surface coverage above the required threshold (`daemon_commands.go` 81.1%, `workspace_config.go` 81.9%).
- Ran `go test ./internal/api/client ./internal/daemon ./internal/cli/...` and `make verify` successfully.

Now:

- Update workflow/task tracking and prepare the final handoff.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.compozy/tasks/daemon/task_11.md`
- `.compozy/tasks/daemon/_techspec.md`
- `.compozy/tasks/daemon/_tasks.md`
- `.compozy/tasks/daemon/memory/{MEMORY.md,task_11.md}`
- `internal/api/client/client.go`
- `internal/daemon/{host.go,task_transport_service.go,run_manager.go}`
- `internal/cli/{daemon.go,daemon_commands.go,daemon_launch_unix.go,daemon_launch_other.go,root.go,state.go,workspace_config.go}`
- `internal/core/workspace/{config_types.go,config_merge.go,config_validate.go}`
- `internal/cli/{commands_test.go,daemon_commands_test.go,root_command_execution_test.go,root_test.go,workspace_config_test.go,testdata/tasks_run_help.golden}`
- Commands: `go test ./internal/api/client ./internal/daemon ./internal/cli/...`, `make verify`
- `internal/cli/root.go`
- `internal/cli/commands.go`
- `internal/cli/state.go`
- `internal/cli/run.go`
- `internal/cli/workspace_config.go`
- `internal/cli/daemon.go`
- `internal/cli/runs.go`
- `internal/cli/commands_simple.go`
- `internal/core/kernel/commands/run_start.go`
- `internal/core/kernel/handlers.go`
