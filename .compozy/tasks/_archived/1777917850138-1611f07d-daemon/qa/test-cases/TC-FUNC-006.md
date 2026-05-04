## TC-FUNC-006: Runs Attach and Watch Operator Flow

**Priority:** P0 (Critical)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 15 minutes
**Created:** 2026-04-18
**Last Updated:** 2026-04-18
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/cli -run 'Test(RunsAttachCommandUsesRemoteUIAttach|RunsWatchCommandStreamsWithoutLaunchingUI)' -count=1`
- Supporting seams:
- `go test ./internal/core/run/ui -run 'Test(RemoteSnapshotBootstrapHydratesUIStateBeforeLiveEvents|FollowRemoteRunReconnectsFromOverflowCursor|AttachRemoteSkipsLiveStreamForCompletedSnapshot|AttachRemoteOpensStreamFromSnapshotCursorForRunningRun|ShouldStopAfterRemoteEOFUsesTerminalSnapshotCursor)' -count=1`
- `go test ./pkg/compozy/runs -run 'Test(WatchRemoteReconnectsAfterOverflowWithoutDuplicatingEvents|WatchRemoteStopsAfterHeartbeatEOFWhenSnapshotIsTerminal)' -count=1`
**Automation Notes:** The CLI lane proves operator-facing attach/watch behavior. The TUI/run-reader seams prove snapshot bootstrap, reconnect, overflow, and terminal EOF correctness.

### Objective

Verify that `compozy runs attach <run-id>` restores daemon-managed runs from snapshot state and `compozy runs watch <run-id>` continues as a textual observation path with reliable reconnect behavior.

### Preconditions

- [ ] A daemon-managed run fixture exists with enough event history to exercise snapshot + live-stream behavior.
- [ ] The environment can run CLI and integration tests that open run streams.

### Test Steps

1. Run the focused CLI attach/watch suite listed above.
   **Expected:** The package exits `0` and attach/watch public flows pass.

2. Run the supporting remote-TUI and run-reader stream suites.
   **Expected:** Snapshot bootstrap happens before live events, overflow reconnect resumes from the last good cursor, terminal EOF stops correctly for terminal runs, and watch mode does not launch the full TUI.

3. Confirm completed-run attach behavior is covered.
   **Expected:** Attaching to a completed run renders the final snapshot without waiting indefinitely on live traffic.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Running run attach | Snapshot plus live stream | Attach resumes from snapshot cursor and continues streaming |
| Completed run attach | Terminal snapshot | Attach exits cleanly without requiring live traffic |
| Overflow reconnect | Stale or overflow cursor | Client reconnects from the last acknowledged cursor without duplicate events |
| Watch-only mode | Text observation path | Events stream without launching TUI |

### Related Test Cases

- `TC-FUNC-002`
- `TC-UI-001`
- `TC-INT-002`

### Traceability

- TechSpec Integration Tests: `compozy runs watch` can reconnect to a persisted run stream; `compozy runs attach` restores from snapshot then continues from cursor.
- ADR-004: TUI-first UX with explicit attach/watch surfaces.
- Task reference: `task_12`.

### Notes

- This case is P0 because attach/watch is the operator-facing observation and recovery contract for daemon-managed runs.
