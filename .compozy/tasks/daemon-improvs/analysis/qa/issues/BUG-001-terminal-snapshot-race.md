# BUG-001: Stream-attached task runs returned before durable terminal snapshot

**Severity:** High
**Priority:** P1
**Type:** Functional
**Status:** Fixed

## Environment

- **Build:** `daemon-improvs` worktree on `2026-04-21`
- **OS:** macOS (local development run)
- **Browser:** N/A
- **URL:** Daemon run snapshot and stream surfaces over HTTP/UDS

## Summary

During the required real-daemon operator-flow validation, `compozy tasks run ... --stream` could exit immediately after the terminal stream event while the daemon still reported the run as non-terminal through the snapshot surface. This made immediate post-run inspection race against durable daemon state.

## Reproduction

```bash
go test ./internal/cli -run 'TestDaemonPublicSnapshotAndStreamMatchAcrossHTTPAndUDSForTempWorkspaceRun' -count=1
```

Observed before the fix:

- The command returned after the terminal stream event.
- An immediate `GetRunSnapshot` still reported `Run.Status == "running"` for the same run.
- Extension-backed exec flows could also return before shutdown-side effects were fully recorded.

## Expected

Once a stream-attached CLI run returns control to the operator, immediate daemon-backed run inspection should already observe the durable terminal snapshot.

## Root cause

`defaultWatchCLIRun` returned as soon as the terminal event arrived from `pkg/compozy/runs.WatchRemote`. That happened before the CLI had waited for the daemon’s durable terminal snapshot path, so public inspection could race the final run-state mirror and teardown.

## Fix

`defaultWatchCLIRun` now records whether a terminal event was observed and, before returning, waits for `waitForTerminalDaemonRunSnapshot(...)` to confirm durable terminal state. Regression coverage was added for both the helper path and the real daemon operator flow.

## Verification

- `go test ./internal/cli -run 'TestDaemonPublicSnapshotAndStreamMatchAcrossHTTPAndUDSForTempWorkspaceRun' -count=1`
- `go test ./internal/cli -run 'TestReviewsExecDaemonStreamHelpers' -count=1`
- `go test ./internal/cli -run 'TestExecCommandWithExtensionsFlagSpawnsWorkspaceExtensionAndWritesAudit' -count=1 -v`
- `make verify`

## Impact

- **Users Affected:** All operators using daemon-backed stream-attached task/exec flows
- **Frequency:** Sometimes, depending on teardown timing
- **Workaround:** None reliable beyond manually polling after the command exited

## Automation Follow-up

- **Required:** Yes
- **Status:** Added
- **Spec / Command:** `internal/cli/operator_transport_integration_test.go`, `internal/cli/reviews_exec_daemon_additional_test.go`
- **Notes:** The live daemon operator-flow test now proves immediate post-run HTTP/UDS snapshot/replay parity, and the helper test locks the durability wait at the CLI stream boundary.

## Related

- Test Case: `TC-FUNC-002`, `TC-INT-005`
- Figma Design: N/A
