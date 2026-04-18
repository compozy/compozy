# Task Memory: task_14.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Complete task 14 by finishing the daemon/operator CLI surface for `daemon`, `workspaces`, top-level `sync`, and top-level `archive`.
- Keep the implementation on top of the task 11 daemon-client foundation and the task 09 DB-backed archive semantics instead of reintroducing direct local execution heuristics.

## Important Decisions
- Treat `daemon status` / `daemon stop` as explicit operator commands that should not auto-start the daemon; they probe the existing daemon state first and use the transport only when an instance is actually present.
- Treat `workspaces`, top-level `sync`, and top-level `archive` as daemon-backed operator flows that should auto-start/reuse the daemon through the shared CLI bootstrap.
- Use daemon task/workspace/sync services for command completion rather than adding new command families or compatibility aliases.
- Preserve `workspaces show|unregister <id-or-path>` semantics in the CLI by resolving path-like refs to registered workspace IDs before calling the current daemon routes; route parameters are safe for IDs but unreliable for raw filesystem paths.
- Keep archive JSON explicitly tagged in snake_case so operator output remains script-friendly for archived and skipped workflow lists.

## Learnings
- The current daemon host only wires `Daemon`, `Tasks` (start run only), and `Runs` services; `Workspaces` and `Sync` transport services are still absent.
- The current CLI still has a help-root `workspaces` command, no `daemon stop`, and top-level `sync` / `archive` still dispatch local core functions instead of the daemon API.
- Real CLI integration tests on macOS need a short daemon home path; long `t.TempDir()` roots can push the derived Unix socket path past platform limits and prevent daemon readiness from being written.
- CLI command tests that build the standalone binary while temporarily overriding `HOME` should build with the original shell `HOME` so the Go module cache remains reusable and startup timings stay stable.
- Workspace and sync assertions need symlink-normalized path comparisons because temp roots can resolve as `/var/...` in the caller and `/private/var/...` in daemon responses.

## Files / Surfaces
- `internal/cli/{daemon.go,daemon_commands.go,commands_simple.go,root.go,workspace_config.go}`
- `internal/api/client/{client.go,runs.go}` plus new daemon/operator client methods as needed
- `internal/api/core/{interfaces.go,handlers.go}`
- `internal/daemon/{host.go,service.go,task_transport_service.go}` plus new transport services/mappers as needed
- `internal/cli/{commands_test.go,daemon_commands_test.go,root_test.go,root_command_execution_test.go,workspace_config_test.go,archive_command_integration_test.go}`
- `internal/cli/{validate_tasks_test.go,operator_commands_integration_test.go,workspace_commands.go}`
- `internal/core/model/workflow_ops.go`

## Errors / Corrections
- The first attempt to pre-start a real daemon inside the CLI integration test process used `daemon.Run` directly, but readiness never reached `ready` in that harness; the stable approach is to let the external CLI commands own daemon auto-start.
- CLI integration failures that reported missing `daemon.json` were traced to overlong Unix socket paths under macOS temp directories, not to the operator command implementations themselves.
- The archive integration test initially exercised `executeRootCommand`, which runs inside `cli.test` and therefore cannot auto-start a real daemon correctly; switching it to the built CLI binary fixed the real-daemon path.

## Ready for Next Run
- Task-14 command behavior, focused integration coverage, workflow memory updates, task tracking updates, and `make verify` are complete; only the required local commit and final handoff remain.
