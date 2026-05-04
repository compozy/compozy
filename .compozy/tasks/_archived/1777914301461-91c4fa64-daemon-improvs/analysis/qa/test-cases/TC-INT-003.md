## TC-INT-003: Runtime Shutdown, Logging Policy, and Checkpoint Discipline

**Priority:** P0 (Critical)
**Type:** Integration
**Status:** Passed
**Estimated Time:** 18 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/daemon -run 'Test(CloseHostRuntimeUsesBoundedContexts|CloseRunScopeUsesBoundedContext|DaemonRunSignalContextDetachedIgnoresCallerCancellation|StopDaemonHTTPReturnsConflictThenForceCancelsActiveRun|RunManagerShutdownHonorsDrainTimeoutAndKeepsTerminalState|RunManagerShutdownWithoutForceReturnsConflictProblem|ManagedDaemonStopEndpointShutsDownAndRemovesSocket|ManagedDaemonRunModesControlLogging)' -count=1`
- `go test ./internal/logger ./internal/store ./internal/store/globaldb ./internal/store/rundb -run 'Test(InstallDaemonLoggerForegroundMirrorsToStderrAndFile|InstallDaemonLoggerDetachedWritesOnlyFile|CloseSQLiteDatabaseCheckpointsBeforeClose|CloseSQLiteDatabaseStillClosesWhenCheckpointFails|GlobalDBCloseContextDelegatesToSQLiteCloser|RunDBCloseContextDelegatesToSQLiteCloser)' -count=1`
**Automation Notes:** This case combines the runtime, logger, and store close-path seams because the feature changes are only trustworthy when shutdown ownership, log sinks, and checkpoints all stay aligned.

### Objective

Verify that daemon lifecycle hardening is durable: shutdown paths use bounded ownership, force-vs-graceful stop behavior remains correct, detached and foreground log policies stay distinct, and SQLite close paths checkpoint before final handle closure.

### Preconditions

- [ ] The daemon runtime tests can start and stop managed runtime state in isolation.
- [ ] Logger file sink creation is allowed in temp directories.
- [ ] The SQLite store packages are runnable in temp-home test environments.

### Test Steps

1. Run the focused daemon runtime/shutdown command listed above.
   **Expected:** Bounded close contexts, detached signal ownership, stop-conflict behavior, and managed-daemon stop/logging semantics all pass.

2. Run the logger/store close-path command listed above.
   **Expected:** Foreground mirroring vs detached file-only logging remains correct, and SQLite closer paths checkpoint before close.

3. Confirm the combined result still supports the public daemon lifecycle checked by `TC-FUNC-001`.
   **Expected:** No contradiction appears between integration seams and operator-visible daemon behavior.

4. Treat any failure as a release blocker for the daemon-hardening bundle.
   **Expected:** The issue is handled as a runtime correctness regression, not a documentation mismatch.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Graceful stop conflict | Stop with active runs and no force | Stable conflict response |
| Forced stop | Stop with force semantics | Active work is canceled and runtime drains within bounds |
| Detached daemon log sink | File sink path unavailable | Clear startup failure for detached mode |
| Foreground daemon logging | Foreground launch | Logs mirror to stderr and file according to policy |
| Checkpoint failure | SQLite checkpoint error | Final close still happens while surfacing the failure |

### Related Test Cases

- `TC-FUNC-001`
- `TC-INT-004`

### Traceability

- TechSpec `Technical Dependencies`
- TechSpec `Monitoring and Observability`
- ADR-002: incremental runtime supervision hardening
- ADR-004: observability as a first-class contract
- Task reference: `task_05.md`

### Notes

- This case exists because shutdown, logging, and store-close behavior form one operational contract. Splitting them apart would make it easier to miss cross-boundary regressions.
