## TC-INT-001: Daemon Bootstrap and Recovery

**Priority:** P0 (Critical)
**Type:** Integration
**Status:** Not Run
**Estimated Time:** 15 minutes
**Created:** 2026-04-18
**Last Updated:** 2026-04-18
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/daemon -run 'Test(StartUsesHomeScopedLayoutFromWorkspaceSubdirectory|StartRecoversAfterKilledDaemonLeavesStaleArtifacts|StartUsesSameHomeScopedDaemonAcrossWorkspaces|StartReconcilesInterruptedRunsBeforeReady|StartRemainsHealthyWhenInterruptedRunDBIsMissingOrCorrupt)' -count=1`
**Automation Notes:** This is the first blocker lane. It proves the singleton daemon boots from `$HOME`, repairs stale artifacts, reuses the same daemon across workspaces, and reconciles interrupted runs before any higher-level operator flow is trusted.

### Objective

Validate the daemon bootstrap contract required by ADR-001 and the TechSpec: one home-scoped singleton per machine, idempotent startup, stale-artifact recovery, and safe reconciliation of interrupted runs.

### Preconditions

- [ ] Repository dependencies are available for `go test`.
- [ ] The execution environment allows isolated temp homes and Unix domain sockets.
- [ ] No unrelated daemon process is reusing the same temporary `$HOME`.

### Test Steps

1. Run the focused daemon bootstrap/recovery suite listed above.
   **Expected:** The package exits `0` and all listed subtests pass.

2. Confirm the suite covers startup from a workspace subdirectory, stale singleton artifact repair, and same-home daemon reuse across multiple workspaces.
   **Expected:** Startup is rooted at the temp `$HOME`, stale lock/info/socket artifacts are repaired, and the same daemon identity is reused for multiple workspaces under one home.

3. Confirm the suite covers interrupted-run reconciliation, including missing or corrupt per-run DB handling.
   **Expected:** Interrupted runs are reconciled before readiness, missing/corrupt `run.db` artifacts do not crash startup, and the daemon surfaces a safe terminal/crashed outcome.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Stale runtime info | Corrupt or stale daemon info/socket files | Bootstrap repairs artifacts and reaches ready state |
| Shared home across workspaces | Two distinct workspaces under one temp `$HOME` | One singleton daemon is reused |
| Interrupted run with missing DB | Reconciliation after crash | Daemon stays healthy and records the failure safely |

### Related Test Cases

- `TC-FUNC-001`
- `TC-INT-002`

### Traceability

- TechSpec Integration Tests: `ensureDaemon()` bootstraps automatically, reuses healthy daemon, and recovers stale singleton artifacts.
- TechSpec Integration Tests: daemon crash mid-run is reconciled on restart.
- ADR-001: global home-scoped singleton daemon.
- Task references: `task_14`, `task_17`.

### Notes

- If this case fails, stop the smoke pass immediately. The rest of the daemon matrix depends on a trustworthy singleton lifecycle.
