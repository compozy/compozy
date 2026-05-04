# Task Memory: task_02.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Completed task_02 by extending the existing shared runtime harness instead of rebuilding it: runtime manifest + transport artifact capture are in place, repeated daemon boot/cleanup is deterministic, and the dedicated runtime lane now executes the harness package itself.

## Important Decisions

- Use `internal/testutil/e2e` as the task baseline; the package already boots a real daemon and is consumed by daemon/http integration suites.
- Keep `make verify` unchanged and improve the separate runtime integration lane instead of broadening the default verification loop.
- Keep the ACP mock-driver override scoped to integration commands only; `mage verify` must run without `AGH_TEST_ACPMOCK_DRIVER_BIN` so `internal/testutil/acpmock` can still validate default driver resolution.

## Learnings

- The shared harness needed semantic SSE event inference because some prompt/exec streams emit JSON `data:` frames without explicit `event:` lines; later transport-parity helpers must consume the normalized event names instead of raw transport labels alone.
- Repeated runtime boots can fail on transient HTTP bind collisions; startup is now stable by capturing process logs, cleaning stale daemon-info/socket artifacts, reseeding the HTTP port, and retrying readiness a bounded number of times.
- The package coverage bar was easiest to preserve by testing stable harness helpers directly (`installRuntimeCLI`, `runtimeRunDirectories`) instead of adding artificial integration work.

## Files / Surfaces

- `internal/testutil/e2e/artifacts.go`
- `internal/testutil/e2e/artifacts_test.go`
- `internal/testutil/e2e/config_seed.go`
- `internal/testutil/e2e/config_seed_test.go`
- `internal/testutil/e2e/runtime_harness.go`
- `internal/testutil/e2e/runtime_harness_helpers_test.go`
- `internal/testutil/e2e/runtime_harness_integration_test.go`
- `internal/testutil/e2e/runtime_harness_test.go`
- `internal/testutil/e2e/runtime_harness_lifecycle_test.go`
- `internal/testutil/e2e/transport_parity.go`
- `internal/testutil/e2e/transport_parity_test.go`
- `internal/e2elane/lanes.go`
- `internal/e2elane/lanes_test.go`

## Errors / Corrections

- Corrected the initial assumption that task_02 started from an empty harness; the repository already contains a substantial partial implementation that needs task-spec alignment rather than duplication.
- Corrected an `errcheck` failure in `cleanupFailedStart` by acknowledging `pollExit` errors without converting expected non-zero exited-process results into secondary cleanup failures.
- Recorded one transient rerun-only failure in `mage testE2ERuntime` (`TestDaemonE2EFixtureBackedMockAgentLaunchesThroughNormalAgentDefinition`); the test passed immediately in isolation and on the next full-lane rerun, so the final verification evidence uses the clean rerun.
- A post-commit `mage verify` rerun on the live checkout later hit unrelated dirty-worktree daemon-interface drift (`fakeSessionManager` missing `ClearConversation`) outside task_02 scope; the task commit itself was already verified before those unrelated changes blocked the live branch recheck.

## Ready for Next Run

- Later tasks can rely on `RuntimeManifest`, `RuntimeManifestPath`, `CaptureTransportOutput`, `CaptureCLIOutput`, and the runtime-lane inclusion of `internal/testutil/e2e` instead of rebuilding daemon boot or artifact capture helpers.
