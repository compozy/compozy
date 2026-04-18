Goal (incl. success criteria):

- Complete daemon-backed CLI command surfaces for `daemon`, `workspaces`, `sync`, and `archive` under task 14.
- Success means: approved command families are wired through daemon APIs, output/exit behavior is stable in text and JSON modes, workspace resolution stays daemon-backed, tests cover help/output/conflicts/integration flows, task/workflow memory is updated, and `make verify` passes.

Constraints/Assumptions:

- Must follow repo instructions from `AGENTS.md` / `CLAUDE.md`, task 14 spec, `_techspec.md`, `_tasks.md`, ADR-001/003/004, and provided workflow memory.
- Required skills loaded/read for this run: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, `systematic-debugging`, `no-workarounds`, `cy-final-verify`. `brainstorming` was read because the task modifies command behavior, but the task spec and accepted techspec are treated as the already-approved design baseline.
- Worktree is dirty in task-tracking and ledger files unrelated to this task; do not revert or disturb unrelated changes.
- Completion requires fresh verification evidence with `make verify`; no destructive git commands.

Key decisions:

- Treat task 14 as command-surface completion on top of the task 11 daemon-client foundation rather than a new transport layer.
- Use the TechSpec command model literally: `daemon start|stop|status`, `workspaces list|show|register|unregister|resolve`, top-level `sync`, and top-level `archive`; no compatibility aliases.
- Keep sync/archive state sourced from daemon/global DB APIs rather than local filesystem heuristics.

State:

- In progress with clean `make verify`, workflow memory updated, and task tracking updated; only code staging and the required local commit remain.

Done:

- Read `AGENTS.md`, `CLAUDE.md`, task 14 spec, `_techspec.md`, `_tasks.md`, ADR-001/003/004, shared workflow memory, current task memory, and cross-agent daemon ledgers relevant to task 14.
- Read required skill guides for workflow memory, task execution, Go implementation, test discipline, debugging/root-cause, workaround avoidance, and final verification.
- Inspected current worktree status to identify unrelated edits that must be left alone.
- Located the relevant TechSpec sections for daemon/workspace/sync/archive command semantics and transport status mapping.
- Confirmed the in-flight task-14 worktree already includes the new daemon/workspace/sync/archive command families plus daemon transport services; avoided reverting or duplicating those edits.
- Fixed CLI test-binary builds to reuse the original shell `HOME` so integration tests do not redownload modules or miss the shared Go cache when command tests temporarily override `HOME`.
- Normalized CLI sync/workspace integration assertions through `filepath.EvalSymlinks` so macOS `/var` vs `/private/var` temp-root aliases do not create false failures.
- Reworked operator integration tests to use real CLI auto-start flows with a short `/tmp`-scoped test `HOME`; long macOS temp paths can exceed Unix socket path limits and prevent daemon readiness from ever being written.
- Fixed `workspaces show|unregister` path-ref handling in the CLI by resolving filesystem refs to registered workspace IDs before calling daemon routes, preserving `id-or-path` command semantics over the current route-parameter transport.
- Added explicit snake_case JSON tags to `core/model.ArchiveResult` so archive command JSON remains script-friendly for archived/skipped path details.
- Verified passing focused coverage for the real-daemon operator suite and surrounding daemon/API suites:
  - `go test ./internal/cli -run 'Test(DaemonStatusAndStopCommandsOperateAgainstRealDaemon|WorkspaceCommandsReflectDaemonRegistryAgainstRealDaemon|WorkspacesUnregisterRejectsActiveRunsAgainstRealDaemon|SyncAndArchiveCommandsUseDaemonStateFromWorkspaceSubdirectory)' -timeout=120s`
  - `go test ./internal/api/...`
  - `go test ./internal/daemon/...`
- Fixed the remaining task-14 archive integration test to use the built CLI binary plus the short-home helper instead of in-process `executeRootCommand`, which cannot auto-start a real daemon from `cli.test`.
- Ran the full repository gate successfully with `make verify` after the task-14 changes.
- Updated workflow memory, `task_14.md`, and `_tasks.md` from the verified state.

Now:

- Stage only the task-14 code/test surfaces (not tracking-only files) and create the required local commit.

Next:

- Report the verified outcome and any residual non-task observations after the local commit is created.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: the broad `go test ./internal/cli/... -timeout=120s` timeout also surfaced a slow existing extension-fixture build (`TestExecCommandWithInstalledWorkspaceExtensionStaysEphemeralWithoutFlag`), but `make verify` completed cleanly and task-14 verification is not blocked.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-18-MEMORY-daemon-command-surface.md`
- `.compozy/tasks/daemon/{_techspec.md,_tasks.md,task_14.md,adrs/adr-001.md,adrs/adr-003.md,adrs/adr-004.md}`
- `.compozy/tasks/daemon/memory/{MEMORY.md,task_14.md}`
- `internal/cli/{root.go,commands.go,workspace_config.go,validate_tasks.go,root_command_execution_test.go,workspace_config_test.go,commands_test.go}`
- `internal/cli/{daemon_commands_test.go,operator_commands_integration_test.go,validate_tasks_test.go,workspace_commands.go}`
- `internal/core/model/workflow_ops.go`
- `internal/core/{archive.go,sync.go,migration/migrate.go,tasks/store.go}`
- `git status --short`
- `rg`
- `sed -n`
- `go test ./internal/cli -run 'Test(DaemonStatusAndStopCommandsOperateAgainstRealDaemon|WorkspaceCommandsReflectDaemonRegistryAgainstRealDaemon|WorkspacesUnregisterRejectsActiveRunsAgainstRealDaemon|SyncAndArchiveCommandsUseDaemonStateFromWorkspaceSubdirectory)' -timeout=120s`
- `go test ./internal/cli -run 'TestArchiveCommandArchivesSyncedWorkflowIntoNewPathFormat' -timeout=120s`
- `go test ./internal/api/...`
- `go test ./internal/daemon/...`
- `make verify`
