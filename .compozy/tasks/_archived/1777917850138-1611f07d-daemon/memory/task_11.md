# Task Memory: task_11.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Convert the CLI foundation into a daemon client for daemon-backed task execution.
- Success criteria: shared daemon bootstrap/handshake, client-side workspace and attach-mode resolution, removal of root `compozy start`, stable transport errors, and clean verification.

## Important Decisions
- Scope task 11 around the daemon-backed `tasks run` path plus root command-surface cleanup, while leaving fuller workspace/reviews/exec/sync/archive command completion to later tasks `14` and `15`.
- Reuse `commandKindStart` state/config plumbing for task workflows instead of duplicating command state, but remove the legacy root `start` entrypoint from `internal/cli/root.go`.
- Keep `workspaces` and `reviews` root commands as daemon-oriented placeholders/help roots for now rather than inventing incomplete direct-execution behavior.

## Learnings
- `runs.default_attach_mode` needed to live in merged workspace/global config so the client could resolve `auto|ui|stream|detach` before contacting the daemon.
- Focused file-level coverage for the task surfaces reached the required threshold after the final test pass: `internal/cli/daemon_commands.go` at `81.1%` and `internal/cli/workspace_config.go` at `81.9%`.
- The daemon client request path needed an explicitly local-only transport implementation (`RoundTripper` + validated `/api/...` path application) to satisfy `gosec` without suppressions.

## Files / Surfaces
- `internal/api/client/client.go`
- `internal/daemon/{host.go,task_transport_service.go,run_manager.go}`
- `internal/cli/{daemon.go,daemon_commands.go,daemon_launch_unix.go,daemon_launch_other.go,root.go,state.go,workspace_config.go}`
- `internal/core/workspace/{config_types.go,config_merge.go,config_validate.go}`
- `internal/cli/{commands_test.go,daemon_commands_test.go,root_command_execution_test.go,root_test.go,workspace_config_test.go,testdata/tasks_run_help.golden}`

## Errors / Corrections
- `make verify` initially failed on lint issues in the new client/bootstrap path:
  - `internal/api/client/client.go` complexity and SSRF warning
  - `internal/daemon/host.go` complexity
  - `internal/cli/daemon_commands.go` `exec.Command` usage
- Fixed by splitting client response handling into helpers, validating daemon request paths and using the underlying `RoundTripper`, extracting host-runtime preparation helpers, and switching daemon launch to `exec.CommandContext(context.Background(), ...)`.

## Ready for Next Run
- Later task `12` can build its remote attach/watch work directly on the new daemon-backed `tasks run` foundation and attach-mode contract.
- Later tasks `14` and `15` should extend the daemon command families from `internal/cli/daemon_commands.go` instead of restoring any direct local execution path.
