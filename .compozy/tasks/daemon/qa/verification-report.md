VERIFICATION REPORT
-------------------
Claim: Daemon QA execution and operator-flow validation for task 19 is complete on the current branch state.
Command: `make verify`
Executed: 2026-04-18T18:33:41Z
Exit code: 0
Output summary: `golangci-lint` reported `0 issues`; repo tests finished with `DONE 2337 tests, 1 skipped`; the build completed and printed `All verification checks passed`.
Warnings: none
Errors: none
Verdict: PASS

AUTOMATED COVERAGE
------------------
Support detected: yes
Harness: generic Go integration and package tests through public daemon-facing CLI/API entrypoints
Canonical command: `none` (the repo uses focused `go test` suites plus `make verify`, not a dedicated single E2E target)
Required flows:
  - Daemon bootstrap and recovery: existing-e2e
  - Workspace registry CLI: existing-e2e
  - Task runs and attach mode: existing-e2e
  - Sync and archive: existing-e2e
  - Review flows: existing-e2e
  - Exec flows: existing-e2e
  - Runs attach/watch: existing-e2e
  - UDS/HTTP parity and SSE resume: existing-e2e
  - `pkg/compozy/runs` compatibility: existing-e2e
  - Performance guardrail: existing-e2e
  - Manual TUI operator confirmation: manual-only
  - Browser/web validation: blocked
Specs added or updated:
  - `internal/core/run/executor/execution_test.go`: added `TestEmitRunStartWaitsForObserverHooksBeforeReturning` so `run.post_start` must drain before `job.pre_execute`.
  - `internal/store/globaldb/registry_test.go`: converted nil-context validation to a typed nil `context.Context` so the repo linter no longer emits staticcheck autofix warnings.
  - `pkg/compozy/runs/transport_test.go`: converted nil-context validation to a typed nil `context.Context` so the repo linter no longer emits staticcheck autofix warnings.
Commands executed:
  - `make deps` | Exit code: 0 | Summary: installed the repo-preferred `gotestsum` and `golangci-lint v2.11.4` toolchain.
  - `make verify` | Exit code: 2 | Summary: baseline gate failed at `internal/core/extension.TestHookDispatchIntegrationAcrossRunAndJobPhases` with `run.post_start` / `job.pre_execute` ordering mismatch.
  - `go test ./internal/core/run/executor -run 'Test(EmitRunStartWaitsForObserverHooksBeforeReturning|FinalizeExecutionWaitsForObserverHooksWithoutCanceledRunContext)' -count=1` | Exit code: 0 | Summary: executor regression coverage for observer draining passed.
  - `go test -race -parallel=4 ./internal/core/extension -count=1` | Exit code: 0 | Summary: extension integration package passed after the hook-order fix.
  - `go test ./internal/daemon -run 'Test(StartUsesHomeScopedLayoutFromWorkspaceSubdirectory|StartRecoversAfterKilledDaemonLeavesStaleArtifacts|StartUsesSameHomeScopedDaemonAcrossWorkspaces|StartReconcilesInterruptedRunsBeforeReady|StartRemainsHealthyWhenInterruptedRunDBIsMissingOrCorrupt)' -count=1` | Exit code: 0 | Summary: `TC-INT-001` passed.
  - `go test ./internal/cli -run 'Test(WorkspaceCommandsReflectDaemonRegistryAgainstRealDaemon|WorkspacesUnregisterRejectsActiveRunsAgainstRealDaemon)' -count=1` | Exit code: 0 | Summary: `TC-FUNC-001` passed.
  - `go test ./internal/cli -run 'Test(TasksRunCommandDispatchesResolvedWorkspaceAndConfiguredAttachMode|TasksRunCommandAutoModeResolvesToStreamInNonInteractiveExecution|TasksRunCommandInteractiveUIModeAttachesThroughRemoteClient|TasksRunCommandExplicitUIFailsWithoutTTY|TasksRunCommandBootstrapFailureReturnsStableExitCode)' -count=1` | Exit code: 0 | Summary: `TC-FUNC-002` passed, then reran after the final gate (`postclean-tc-func-002.log`) and stayed green.
  - `go test ./internal/cli -run 'Test(SyncAndArchiveCommandsUseDaemonStateFromWorkspaceSubdirectory|ArchiveCommandArchivesSyncedWorkflowIntoNewPathFormat)' -count=1` | Exit code: 0 | Summary: `TC-FUNC-003` passed.
  - `go test ./internal/cli -run 'Test(ReviewsCommandFetchListShowUseDaemonRequests|ReviewsFixCommandResolvesLatestRoundAndBuildsDaemonRequest|ReviewsFixCommandAutoAttachStreamsWhenNonInteractive)' -count=1` | Exit code: 0 | Summary: `TC-FUNC-004` passed.
  - `go test ./internal/cli -run 'Test(ExecCommandUsesDaemonLifecycleAcrossFormats|ExecCommandExecuteStdinWorksEndToEnd|ExecCommandExecutePromptFileJSONEmitsJSONLByDefault|ExecCommandExecuteRunIDUsesPersistedRuntimeDefaults)' -count=1` | Exit code: 0 | Summary: `TC-FUNC-005` passed.
  - `go test ./internal/cli -run 'Test(RunsAttachCommandUsesRemoteUIAttach|RunsWatchCommandStreamsWithoutLaunchingUI)' -count=1` | Exit code: 0 | Summary: `TC-FUNC-006` passed.
  - `go test ./internal/api/httpapi -run 'Test(HTTPAndUDSRegisterMatchingRoutes|HTTPServerPersistsActualPortInDaemonInfo|UDSServerCreates0600Socket|HealthTransitionsOverHTTP|HTTPAndUDSServeMatchingStatusSnapshotAndConflict|HTTPStreamResumesAfterLastEventIDAndEmitsHeartbeat|HTTPStreamRejectsInvalidAndStaleCursor)' -count=1` | Exit code: 0 | Summary: `TC-INT-002` passed, then reran after the final gate (`postclean-tc-int-002.log`) and stayed green.
  - `go test ./pkg/compozy/runs -run 'Test(OpenAndListUseDaemonBackedHTTPTransport|TailAndWatchWorkspaceUseDaemonBackedHTTPTransport|OpenLoadsDaemonBackedRunSummary|ListReturnsRunsSortedAndFiltered|WatchWorkspaceEmitsCreatedStatusChangedAndRemoved|WatchWorkspaceSurfacesInitialDaemonError|OpenSurfacesStableDaemonUnavailableError)' -count=1` | Exit code: 0 | Summary: `TC-INT-003` passed.
  - `go test ./internal/daemon -run 'Test(RunManagerTaskRunWatcherSyncsTaskEditsAndStopsOnCancel|RunManagerReviewRunWatcherSyncsOwnedWorkflowArtifacts|RunManagerExecRunCompletesAndReplaysPersistedStream|RunManagerExecRunFailureMarksRunFailed)' -count=1` | Exit code: 0 | Summary: daemon lifecycle supporting seams passed.
  - `go test ./internal/core -run 'Test(SyncTaskMetadataSyncsSingleWorkflowIntoGlobalDBWithoutMutatingArtifacts|SyncTaskMetadataRemovesLegacyGeneratedMetadataOnce|ArchiveTaskWorkflowRejectsPendingStateFromSyncedDBEvenWithStaleMeta|ArchiveTaskWorkflowRejectsActiveRunConflict|ArchiveTaskWorkflowsRootScanUsesDBStateAndSortsSkippedPaths)' -count=1` | Exit code: 0 | Summary: sync/archive supporting seams passed.
  - `go test ./internal/api/core -run 'Test(StreamRunRejectsInvalidLastEventID|StreamRunEmitsHeartbeatAndOverflowFrames|StreamRunAdditionalBranches)' -count=1` | Exit code: 0 | Summary: transport-core SSE edge cases passed.
  - `go test ./pkg/compozy/runs -run 'Test(WatchRemoteReconnectsAfterOverflowWithoutDuplicatingEvents|WatchRemoteStopsAfterHeartbeatEOFWhenSnapshotIsTerminal|TestOpenRunStreamParsesHeartbeatOverflowAndEvents)' -count=1` | Exit code: 0 | Summary: run-reader reconnect/overflow stream seams passed.
  - `go test ./internal/core/run/ui -run 'Test(RemoteSnapshotBootstrapHydratesUIStateBeforeLiveEvents|FollowRemoteRunReconnectsFromOverflowCursor|AttachRemoteSkipsLiveStreamForCompletedSnapshot|AttachRemoteOpensStreamFromSnapshotCursorForRunningRun|ShouldStopAfterRemoteEOFUsesTerminalSnapshotCursor)' -count=1` | Exit code: 0 | Summary: daemon-backed TUI attach/bootstrap/reconnect seams passed.
  - `go test ./internal/core/run/executor -run 'Test(ExecutorWaitsForUIQuitAfterJobsComplete|ExecutorControllerFinalizesNormalCompletionBeforeUIExit|EnsureRuntimeEventBusCreatesFallbackBusForUI|ExecutorControllerUIQuitEntersDrainingPath|ExecutorControllerSuppressesFallbackShutdownLogsWhileUIIsActive)' -count=1` | Exit code: 0 | Summary: executor UI lifecycle seam passed.
  - `go test ./internal/store/rundb ./internal/daemon -run '^$' -bench 'BenchmarkRunDBListEventsFromCursor|BenchmarkRunManagerListWorkspaceRuns' -benchmem -count=1` | Exit code: 0 | Summary: `BenchmarkRunDBListEventsFromCursor` measured `165288 ns/op`, `241640 B/op`, `4134 allocs/op`; `BenchmarkRunManagerListWorkspaceRuns` measured `341317 ns/op`, `179314 B/op`, `4491 allocs/op`, which stays aligned with the task-16 ledger baseline.
  - `hyperfine --warmup 3 --runs 10 './bin/compozy --help > /dev/null' './bin/compozy --version > /dev/null' './bin/compozy completion bash > /dev/null'` | Exit code: 0 | Summary: help/version/completion measured `8.5 ms`, `8.4 ms`, and `8.2 ms` mean runtime respectively; absolute timings remain in the same sub-10 ms envelope as task 16.
  - `make verify` | Exit code: 0 | Summary: final clean repo gate passed with no lint warnings, `2337` tests, `1` skip, and successful build output.
Manual-only or blocked:
  - `TC-UI-001`: manual-only. This run exercised the automated TUI seams (`tui-remote.log`, `tui-executor.log`), but it did not perform a human real-terminal acceptance session because the repo has no stable full-screen terminal harness.
  - Browser/web validation: blocked/out of scope because this branch exposes no daemon web UI surface and the repo has no browser E2E harness.

TEST CASE COVERAGE (when qa-report artifacts exist)
----------------------------------------------------------
Test cases found: 11
Executed: 10
Results:
  - `TC-INT-001`: PASS | Bug: none
  - `TC-FUNC-001`: PASS | Bug: none
  - `TC-FUNC-002`: PASS | Bug: `BUG-001`
  - `TC-FUNC-003`: PASS | Bug: none
  - `TC-FUNC-004`: PASS | Bug: none
  - `TC-FUNC-005`: PASS | Bug: none
  - `TC-FUNC-006`: PASS | Bug: none
  - `TC-INT-002`: PASS | Bug: none
  - `TC-INT-003`: PASS | Bug: none
  - `TC-PERF-001`: PASS | Bug: none
Not executed: `TC-UI-001` (manual-only real-terminal confirmation requires human judgment; automated daemon-backed TUI seams were executed instead)

ISSUES FILED
-------------
Total: 1
By severity:
  - Critical: 0
  - High: 1
  - Medium: 0
  - Low: 0
Details:
  - `BUG-001`: `run.post_start` observers can trail `job.pre_execute` | Severity: High | Priority: P1 | Status: Fixed
