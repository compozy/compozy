## TC-INT-003: Public `pkg/compozy/runs` Compatibility

**Priority:** P1 (High)
**Type:** Integration
**Status:** Not Run
**Estimated Time:** 12 minutes
**Created:** 2026-04-18
**Last Updated:** 2026-04-18
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./pkg/compozy/runs -run 'Test(OpenAndListUseDaemonBackedHTTPTransport|TailAndWatchWorkspaceUseDaemonBackedHTTPTransport|OpenLoadsDaemonBackedRunSummary|ListReturnsRunsSortedAndFiltered|WatchWorkspaceEmitsCreatedStatusChangedAndRemoved|WatchWorkspaceSurfacesInitialDaemonError|OpenSurfacesStableDaemonUnavailableError)' -count=1`
- Supporting seam: `go test ./pkg/compozy/runs -run 'Test(WatchRemoteReconnectsAfterOverflowWithoutDuplicatingEvents|WatchRemoteStopsAfterHeartbeatEOFWhenSnapshotIsTerminal|OpenRunStreamParsesHeartbeatOverflowAndEvents)' -count=1`
**Automation Notes:** This case protects the daemon-backed public run-reader API against regressions that would reintroduce workspace-local runtime assumptions.

### Objective

Verify that `pkg/compozy/runs` remains daemon-backed for list/open/tail/watch consumers while preserving stable summaries, ordering, error behavior, and stream semantics.

### Preconditions

- [ ] The package-level integration tests can start their daemon-backed fixtures.
- [ ] The execution branch still treats direct workspace `.compozy/runs` inspection as non-authoritative.

### Test Steps

1. Run the focused public run-reader suite listed above.
   **Expected:** The package exits `0` and all public list/open/watch compatibility tests pass.

2. Confirm the suite covers daemon-backed open/list over HTTP transport, watch events, stable unavailable-daemon errors, and summary ordering.
   **Expected:** Public consumers retain the same ergonomic API shape without direct SQLite or workspace-runtime reads.

3. Run the supporting remote-stream suite when replay/tail/overflow details need deeper evidence.
   **Expected:** Heartbeat, overflow, reconnect, and terminal-stop semantics remain stable.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Daemon unavailable | Reader without daemon | Stable daemon-unavailable error, no silent filesystem fallback |
| Workspace watch | Public watch over daemon events | Created/status-changed/removed surfaces remain stable |
| Overflow reconnect | Long-running stream | Replay resumes without duplicated events |
| Summary ordering | Multiple runs | Newest-first ordering remains stable |

### Related Test Cases

- `TC-FUNC-006`
- `TC-INT-002`

### Traceability

- TechSpec Integration Tests: `pkg/compozy/runs` uses daemon-backed snapshot and stream APIs without reading SQLite directly.
- ADR-002: operational state is daemon-owned, not workspace-runtime truth.
- ADR-003: public readers consume the transport contract.
- Task reference: `task_13`.

### Notes

- If this case fails, treat it as a public compatibility regression even if the CLI smoke suite is still green.
