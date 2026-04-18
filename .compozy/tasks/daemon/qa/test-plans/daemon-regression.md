# Daemon Regression Suite

## Purpose

This suite turns the daemon QA plan into an execution order for `task_19`. It groups the daemon-critical flows into smoke, targeted, and full passes while keeping one artifact root and one final gate: `make verify`.

## Execution Rules

1. Run the smoke suite first. If any `P0` smoke case fails, stop and fix before continuing.
2. Run the targeted suite next for the daemon surface changed or fixed in the current branch.
3. Run the full suite before closing `task_19`, including the repository gate and the transport parity lane.
4. After any bug fix, rerun the narrow failing case, then the affected suite, then `make verify`.
5. Browser validation remains blocked/out of scope on this branch unless a real daemon web UI and harness are added during execution.

## Priority Bands

- `P0`: daemon bootstrap/recovery, workspace registry, task runs, sync/archive, attach/watch, transport parity.
- `P1`: review runs, exec runs, external temp-workspace operator flow, public `pkg/compozy/runs`, performance guardrails, manual TUI operator confirmation.
- `P2`: any future daemon web/browser surface once it exists.

## Smoke Suite

**Goal:** prove the daemon control plane is healthy enough for deeper execution.

**Stop condition:** any `P0` failure blocks the rest of the suite.

| Order | Case ID | Priority | Flow | Lane | Command / Spec |
|---|---|---|---|---|---|
| 1 | `TC-INT-001` | P0 | Daemon bootstrap and recovery | Integration | `go test ./internal/daemon -run 'Test(StartUsesHomeScopedLayoutFromWorkspaceSubdirectory|StartRecoversAfterKilledDaemonLeavesStaleArtifacts|StartUsesSameHomeScopedDaemonAcrossWorkspaces|StartReconcilesInterruptedRunsBeforeReady|StartRemainsHealthyWhenInterruptedRunDBIsMissingOrCorrupt)' -count=1` |
| 2 | `TC-FUNC-001` | P0 | Workspace registry CLI | E2E | `go test ./internal/cli -run 'Test(WorkspaceCommandsReflectDaemonRegistryAgainstRealDaemon|WorkspacesUnregisterRejectsActiveRunsAgainstRealDaemon)' -count=1` |
| 3 | `TC-FUNC-002` | P0 | Task runs, attach mode, watcher sync | E2E | `go test ./internal/cli -run 'Test(TasksRunCommandDispatchesResolvedWorkspaceAndConfiguredAttachMode|TasksRunCommandAutoModeResolvesToStreamInNonInteractiveExecution|TasksRunCommandInteractiveUIModeAttachesThroughRemoteClient|TasksRunCommandExplicitUIFailsWithoutTTY|TasksRunCommandBootstrapFailureReturnsStableExitCode)' -count=1` |
| 4 | `TC-FUNC-003` | P0 | Sync and archive | E2E | `go test ./internal/cli -run 'TestSyncAndArchiveCommandsUseDaemonStateFromWorkspaceSubdirectory|TestArchiveCommandArchivesSyncedWorkflowIntoNewPathFormat' -count=1` |
| 5 | `TC-FUNC-006` | P0 | Attach/watch operator flow | E2E | `go test ./internal/cli -run 'Test(RunsAttachCommandUsesRemoteUIAttach|RunsWatchCommandStreamsWithoutLaunchingUI)' -count=1` |
| 6 | `TC-INT-002` | P0 | UDS/HTTP parity and SSE resume | Integration | `go test ./internal/api/httpapi -run 'Test(HTTPAndUDSRegisterMatchingRoutes|HealthTransitionsOverHTTP|HTTPAndUDSServeMatchingStatusSnapshotAndConflict|HTTPStreamResumesAfterLastEventIDAndEmitsHeartbeat|HTTPStreamRejectsInvalidAndStaleCursor)' -count=1` |

## Targeted Suite

**Goal:** validate the daemon slices most likely to regress after a focused fix or follow-up change.

**Use when:** a branch changes one or more daemon control-plane packages, or a smoke-case bug was fixed.

| Order | Case ID | Priority | Flow | Lane | Command / Spec |
|---|---|---|---|---|---|
| 1 | `TC-FUNC-004` | P1 | Review runs | E2E | `go test ./internal/cli -run 'Test(ReviewsCommandFetchListShowUseDaemonRequests|ReviewsFixCommandResolvesLatestRoundAndBuildsDaemonRequest|ReviewsFixCommandAutoAttachStreamsWhenNonInteractive)' -count=1` |
| 2 | `TC-INT-004` | P1 | Temporary Node workspace task/review operator flow | E2E | `go test ./internal/cli -run 'TestTaskAndReviewCommandsExecuteDryRunAgainstTempNodeWorkspace' -count=1` plus live CLI logs under `.compozy/tasks/daemon/qa/logs/node-e2e-*.log` |
| 3 | `TC-FUNC-005` | P1 | Exec runs | E2E | `go test ./internal/cli -run 'Test(ExecCommandUsesDaemonLifecycleAcrossFormats|ExecCommandExecuteStdinWorksEndToEnd|ExecCommandExecutePromptFileJSONEmitsJSONLByDefault|ExecCommandExecuteRunIDUsesPersistedRuntimeDefaults)' -count=1` |
| 4 | `TC-INT-003` | P1 | Public `pkg/compozy/runs` compatibility | Integration | `go test ./pkg/compozy/runs -run 'Test(OpenAndListUseDaemonBackedHTTPTransport|TailAndWatchWorkspaceUseDaemonBackedHTTPTransport|OpenLoadsDaemonBackedRunSummary|ListReturnsRunsSortedAndFiltered|WatchWorkspaceEmitsCreatedStatusChangedAndRemoved|WatchWorkspaceSurfacesInitialDaemonError|OpenSurfacesStableDaemonUnavailableError)' -count=1` |
| 5 | `TC-PERF-001` | P1 | Performance-sensitive daemon seams | Integration | `go test ./internal/store/rundb ./internal/daemon -run '^$' -bench 'BenchmarkRunDBListEventsFromCursor|BenchmarkRunManagerListWorkspaceRuns' -benchmem -count=1` |
| 6 | `TC-UI-001` | P1 | Manual TUI operator confirmation | Manual-only | Real-terminal execution of `compozy tasks run <slug> --ui` plus `compozy runs attach <run-id>` against a fixture workflow |

## Full Suite

**Goal:** release-level daemon validation with public flows, supporting integration seams, and the repository gate.

**Pass condition:** all `P0` pass, at least 90% of `P1` pass, no critical daemon bug remains open, and `make verify` passes.

| Order | Scope | Required Cases / Commands |
|---|---|---|
| 1 | Smoke prerequisite | Run every smoke-suite item in listed order |
| 2 | Supporting daemon state seams | `go test ./internal/daemon -run 'Test(RunManagerTaskRunWatcherSyncsTaskEditsAndStopsOnCancel|RunManagerReviewRunWatcherSyncsOwnedWorkflowArtifacts|RunManagerExecRunCompletesAndReplaysPersistedStream|RunManagerExecRunFailureMarksRunFailed)' -count=1` |
| 3 | Supporting sync/archive seams | `go test ./internal/core -run 'Test(SyncTaskMetadataSyncsSingleWorkflowIntoGlobalDBWithoutMutatingArtifacts|SyncTaskMetadataRemovesLegacyGeneratedMetadataOnce|ArchiveTaskWorkflowRejectsPendingStateFromSyncedDBEvenWithStaleMeta|ArchiveTaskWorkflowRejectsActiveRunConflict|ArchiveTaskWorkflowsRootScanUsesDBStateAndSortsSkippedPaths)' -count=1` |
| 4 | Transport core edge cases | `go test ./internal/api/core -run 'Test(StreamRunRejectsInvalidLastEventID|StreamRunEmitsHeartbeatAndOverflowFrames|StreamRunAdditionalBranches)' -count=1` |
| 5 | Public run-reader stream behavior | `go test ./pkg/compozy/runs -run 'Test(WatchRemoteReconnectsAfterOverflowWithoutDuplicatingEvents|WatchRemoteStopsAfterHeartbeatEOFWhenSnapshotIsTerminal|TestOpenRunStreamParsesHeartbeatOverflowAndEvents)' -count=1` |
| 6 | Manual TUI lane | Execute `TC-UI-001` and capture notes/screenshots only if meaningful; do not invent a browser lane |
| 7 | Repository gate | `make verify` |

## Explicit P0 / P1 Mapping

| Case ID | Priority | Notes |
|---|---|---|
| `TC-INT-001` | P0 | First blocker: daemon cannot be trusted until singleton bootstrap and reconciliation are green |
| `TC-FUNC-001` | P0 | Workspace identity is a control-plane prerequisite for all higher-level flows |
| `TC-FUNC-002` | P0 | `tasks run` is the canonical daemon-native operator surface |
| `TC-FUNC-003` | P0 | Sync/archive must remain daemon-backed and metadata-free |
| `TC-FUNC-006` | P0 | Attach/watch is the operator-facing observation contract |
| `TC-INT-002` | P0 | Transport divergence invalidates CLI/TUI/API parity claims |
| `TC-FUNC-004` | P1 | Review flows are critical, but not the first blocker ahead of core run lifecycle |
| `TC-INT-004` | P1 | External temp-workspace proof catches regressions that repository-native fixtures can miss |
| `TC-FUNC-005` | P1 | Exec compatibility matters after core task-run health is proven |
| `TC-INT-003` | P1 | Public run readers are a compatibility surface rather than the main operator entrypoint |
| `TC-PERF-001` | P1 | Performance must not regress silently after functional health is green |
| `TC-UI-001` | P1 | Human terminal judgment remains important but should not block initial smoke automation |

## Blocked and Manual-Only Notes

- **Browser validation:** blocked/out of scope. This branch has no daemon web UI surface and no browser E2E harness.
- **TUI visual polish:** manual-only adjunct. Automated daemon/TUI state correctness already exists, but a real-terminal feel/readability check still needs human judgment.
- **CLI cold-start timing adjunct:** if `hyperfine` is unavailable during execution, record the CLI timing check as an environment-limited note without failing the core regression suite.

## Evidence Output

- Record each executed command and result in `.compozy/tasks/daemon/qa/verification-report.md`.
- File any discovered daemon issue as `.compozy/tasks/daemon/qa/issues/BUG-*.md` and reference the originating case ID.
- Use `.compozy/tasks/daemon/qa/screenshots/` only for manual terminal evidence that materially helps root-cause or handoff.
