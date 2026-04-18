Goal (incl. success criteria):

- Complete daemon task `16` by closing migration regressions across tests, docs, help fixtures, and stale legacy references.
- Success means: daemon-backed CLI/TUI/sync/archive/reviews/exec/public-reader flows have updated regression coverage, user-facing docs match the daemon runtime and command surface, stale `compozy start` / `_meta.md` / generated `_tasks.md` references are removed or corrected, workflow memory and tracking are updated, and `make verify` passes before commit.

Constraints/Assumptions:

- Must follow `AGENTS.md`, `CLAUDE.md`, `.compozy/tasks/daemon/task_16.md`, `_techspec.md`, `_tasks.md`, ADR-003/004, and workflow memory files.
- Required skills read for this run: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, `systematic-debugging`, `no-workarounds`, `cy-final-verify`.
- Worktree is already dirty in unrelated daemon task-tracking and memory files from task 15; do not revert or disturb unrelated changes.
- Shared workflow memory was flagged for compaction and must be compacted before implementation.
- Completion requires fresh `make verify` and no destructive git commands.

Key decisions:

- Treat task 16 as migration closeout, not a new feature: prefer extending existing tests and deleting stale references over introducing new harnesses.
- Keep scope on the highest-risk seams called out by the task and techspec: CLI help/output, migration cleanup, daemon-backed exec/review flows, public run-reader behavior, and user-facing docs.
- Preserve compatibility artifacts such as persisted `run.json` / `events.jsonl` only where the daemon migration already intentionally kept them; remove legacy command/docs references instead of adding new shims.

State:

- Completed after fresh `make verify`, tracking updates, and the scoped local commit `2aea6ab` (`test: finalize daemon migration cleanup`).

Done:

- Read the required skills, repo guidance, task spec, `_techspec.md`, `_tasks.md`, ADRs, workflow memory files, and adjacent daemon ledgers.
- Inspected `git status --short` to identify unrelated dirty files that must be left alone.
- Located the main pre-change gaps:
  - `README.md` still documents `compozy start`, `_meta.md`, and pre-daemon execution wording.
  - `docs/extensibility/architecture.md` still references workspace-local `.compozy/runs/<run-id>/extensions.jsonl`.
  - `internal/cli/testdata/start_help.golden` is stale.
  - `internal/core/migration/migrate_test.go` still exercises legacy `_meta.md` migration expectations.
  - Task 16 tracking is still pending.
- Compacted shared workflow memory and initialized task-local memory for task 16.
- Updated user-facing docs to the daemon-native surface:
  - `README.md` now documents the home-scoped daemon model, canonical `tasks run` / `reviews` / `runs` / `workspaces` commands, attach/watch behavior, and compatibility aliases.
  - `docs/reader-library.md` and `docs/events.md` now describe daemon-backed run inspection accurately.
  - extension docs now point at `~/.compozy/runs/<run-id>/run.db` (`hook_runs`) as the durable audit surface and use daemon-valid example commands.
- Removed the orphaned `internal/cli/testdata/start_help.golden` fixture.
- Added regression coverage for:
  - canonical review-fix auto-stream attach behavior in `internal/cli/reviews_exec_daemon_additional_test.go`
  - daemon doc drift in `internal/cli/root_test.go`
  - migration skipping of legacy `_meta.md` / `_tasks.md` artifacts in `internal/core/migration/migrate_test.go`
  - updated command-path expectations in CLI extension/bootstrap skill tests
- Ran focused validation successfully:
  - `go test ./internal/cli -run 'Test(ExecHelpMatchesGolden|READMEExecDocumentationMatchesCurrentContract|ActiveDocsAndHelpFixturesOmitLegacyArtifactRoot|DaemonDocsUseCurrentCommandSurface|TasksRunHelpMatchesGolden|ReviewsCommandFetchListShowUseDaemonRequests|ReviewsFixCommandResolvesLatestRoundAndBuildsDaemonRequest|ReviewsFixCommandAutoAttachStreamsWhenNonInteractive)$'`
  - `go test ./internal/cli -run 'Test(ExecCommandExecuteDirectPromptIsEphemeralByDefault|TasksRunCommandDispatchesResolvedWorkspaceAndConfiguredAttachMode|TasksRunCommandAutoModeResolvesToStreamInNonInteractiveExecution|TasksRunCommandInteractiveUIModeAttachesThroughRemoteClient|TasksRunCommandExplicitUIFailsWithoutTTY|TasksRunCommandBootstrapFailureReturnsStableExitCode|RunsAttachCommandUsesRemoteUIAttach|RunsWatchCommandStreamsWithoutLaunchingUI|FixReviewsCommandExecuteDryRunPersistsKernelArtifacts|FixReviewsCommandExecuteDryRunRawJSONStreamsCanonicalEvents)$'`
  - `go test ./internal/core/migration -run 'Test(MigrateMixedDirectoryCountsV1ToV2AndTracksUnmappedTypes|MigrateSkipsLegacyMetadataAndMasterTaskArtifacts)$'`
  - `go test ./internal/core/run/executor`
  - `go test ./pkg/compozy/runs`
- Ran full `make verify` successfully after the task-16 changes.
- Updated `task_16.md` and `.compozy/tasks/daemon/_tasks.md` to completed.
- Created the scoped local commit `2aea6ab` (`test: finalize daemon migration cleanup`) with only task-16 docs/test/cleanup surfaces staged.

Now:

- Prepare the final handoff with verification evidence and the remaining unstaged workflow/tracking files.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-18-MEMORY-regression-cleanup.md`
- `.compozy/tasks/daemon/{task_16.md,_techspec.md,_tasks.md,memory/MEMORY.md,memory/task_16.md}`
- `.compozy/tasks/daemon/adrs/{adr-003.md,adr-004.md}`
- `README.md`
- `docs/{events.md,reader-library.md}`
- `docs/extensibility/{architecture.md,host-api-reference.md,trust-and-enablement.md,getting-started.md,hello-world-go.md,hello-world-ts.md}`
- `internal/cli/{root_command_execution_test.go,testdata/exec_help.golden,testdata/start_help.golden,testdata/tasks_run_help.golden}`
- `internal/cli/{root_test.go,reviews_exec_daemon_additional_test.go,extensions_bootstrap_test.go,skills_preflight_test.go,form_test.go}`
- `internal/core/migration/migrate_test.go`
- `internal/core/run/executor/execution_acp_integration_test.go`
- `pkg/compozy/runs/integration_test.go`
- Commands: `rg`, `sed -n`, `git status --short`, focused `go test`, `make verify`
