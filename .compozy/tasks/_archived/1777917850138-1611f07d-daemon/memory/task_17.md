# Task Memory: task_17.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Close the daemon migration by expanding regression evidence, aligning user-facing docs with the daemon runtime, and removing stale references to `compozy start`, generated `_tasks.md`, and `_meta.md`.
- Success requires focused unit/integration coverage updates on the daemon-backed CLI/TUI/sync/archive/reviews/exec/public-reader seams plus a final clean `make verify`.

## Important Decisions
- Treat this task as cleanup and evidence-gathering, not new feature work: extend existing suites and docs, delete stale references, and avoid introducing new compatibility layers.
- Document daemon-native commands (`tasks run`, `reviews fix`, `daemon`, `workspaces`, `runs attach|watch`) as canonical, while keeping top-level `fetch-reviews` / `fix-reviews` README sections only as compatibility aliases.
- Keep extension audit documentation aligned with the daemon design by pointing users to `~/.compozy/runs/<run-id>/run.db` (`hook_runs`) instead of the old workspace-local `extensions.jsonl` story.

## Learnings
- `README.md` still contains multiple legacy `compozy start` / `_meta.md` references, and `docs/extensibility/architecture.md` still points at workspace-local extension runtime artifacts.
- `internal/cli/testdata/start_help.golden` is stale relative to the daemon command surface.
- `internal/core/migration/migrate_test.go` still asserts legacy `_meta.md` migration behavior and will likely need cleanup coverage updates.
- The README closeout needed both command-surface additions (`daemon`, `workspaces`, `tasks`, `reviews`, `runs`) and compatibility guidance so the documented CLI matches the actual root command tree instead of only swapping command names inline.
- A small compile regression in the new review stream test came from a missing `io` import; after adding it, focused daemon CLI/migration/executor/public-reader tests and `make verify` all passed.

## Files / Surfaces
- `README.md`
- `docs/extensibility/architecture.md`
- `docs/extensibility/{host-api-reference.md,trust-and-enablement.md,getting-started.md,hello-world-go.md,hello-world-ts.md}`
- `docs/{events.md,reader-library.md}`
- `internal/cli/root_command_execution_test.go`
- `internal/cli/{root_test.go,reviews_exec_daemon_additional_test.go,extensions_bootstrap_test.go,skills_preflight_test.go,form_test.go}`
- `internal/cli/testdata/{start_help.golden,exec_help.golden,tasks_run_help.golden}`
- `internal/core/migration/migrate_test.go`
- `internal/core/run/executor/execution_acp_integration_test.go`
- `pkg/compozy/runs/integration_test.go`

## Errors / Corrections
- `.compozy/tasks/daemon/_meta.md` remains dirty in the worktree from another run and stayed out of this task's implementation/verification/commit scope.
- Added the missing `io` import in `internal/cli/reviews_exec_daemon_additional_test.go` after the first focused `go test ./internal/cli ...` build failed.

## Ready for Next Run
- Focused verification completed successfully:
  - `go test ./internal/cli -run 'Test(ExecHelpMatchesGolden|READMEExecDocumentationMatchesCurrentContract|ActiveDocsAndHelpFixturesOmitLegacyArtifactRoot|DaemonDocsUseCurrentCommandSurface|TasksRunHelpMatchesGolden|ReviewsCommandFetchListShowUseDaemonRequests|ReviewsFixCommandResolvesLatestRoundAndBuildsDaemonRequest|ReviewsFixCommandAutoAttachStreamsWhenNonInteractive)$'`
  - `go test ./internal/cli -run 'Test(ExecCommandExecuteDirectPromptIsEphemeralByDefault|TasksRunCommandDispatchesResolvedWorkspaceAndConfiguredAttachMode|TasksRunCommandAutoModeResolvesToStreamInNonInteractiveExecution|TasksRunCommandInteractiveUIModeAttachesThroughRemoteClient|TasksRunCommandExplicitUIFailsWithoutTTY|TasksRunCommandBootstrapFailureReturnsStableExitCode|RunsAttachCommandUsesRemoteUIAttach|RunsWatchCommandStreamsWithoutLaunchingUI|FixReviewsCommandExecuteDryRunPersistsKernelArtifacts|FixReviewsCommandExecuteDryRunRawJSONStreamsCanonicalEvents)$'`
  - `go test ./internal/core/migration -run 'Test(MigrateMixedDirectoryCountsV1ToV2AndTracksUnmappedTypes|MigrateSkipsLegacyMetadataAndMasterTaskArtifacts)$'`
  - `go test ./internal/core/run/executor`
  - `go test ./pkg/compozy/runs`
- `make verify`
- Task tracking is updated in `task_17.md` (formerly `task_16.md`) and `.compozy/tasks/daemon/_tasks.md`.
- Scoped local commit created: `2aea6ab` (`test: finalize daemon migration cleanup`).
- No implementation work remains for task 17 (formerly task 16); only unstaged workflow memory, task trackers, and session ledgers remain outside the commit by design.
