---
status: resolved
file: internal/cli/run_observe.go
line: 34
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57_io_,comment:PRRC_kwDORy7nkc65JlV1
---

# Issue 001: _🛠️ Refactor suggestion_ | _🟠 Major_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_

**Replace the attach warmup sleep loop with configurable, context-aware polling.**

This path hardcodes the retry policy and uses `time.Sleep`, so shutdown is only observed between sleeps. Please move these timings behind config/options and drive the loop with a ticker plus `select` on `ctx.Done()`. As per coding guidelines, "NEVER use `time.Sleep()` in orchestration — use proper synchronization primitives instead" and "NEVER hardcode configuration — use TOML config or functional options".



Also applies to: 137-168

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/run_observe.go` around lines 30 - 34, The current attach warmup
sleep loop should be replaced with a context-aware polling loop driven by a
time.Ticker and select on ctx.Done(), and the hardcoded timings
(defaultUIAttachSnapshotTimeout, defaultUIAttachSnapshotPollInterval,
defaultOwnedRunCancelTimeout) must be moved into configurable options (exposed
via functional options or TOML-backed config) so callers can override them;
locate the sleep-based warmup loop(s) (the block referenced and the similar code
at lines 137-168) and: (1) add fields or option params for attachSnapshotTimeout
and attachSnapshotPollInterval, (2) swap time.Sleep calls for a ticker-based
loop that checks ctx.Done() in a select and breaks when condition met or ctx
canceled, and (3) use the configurable values instead of the hardcoded constants
throughout the attach/cancel logic.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:1045b61e-8c22-45e4-bd74-0cd183cf33f2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `loadUIAttachSnapshot` was still polling with `time.Sleep(pollInterval)`, so cancellation was only observed before each sleep and not while the helper was blocked waiting for the next retry. The attach and owned-run cancel paths also still bound their timing to the file-level defaults `defaultUIAttachSnapshotTimeout`, `defaultUIAttachSnapshotPollInterval`, and `defaultOwnedRunCancelTimeout` with no override path for callers.
- Resolution: `internal/cli/run_observe.go` now centralizes these timings behind internal functional options, threads the selected values through attach warmup and owned-run cancel, and replaces the warmup sleep loop with a `timer`/`ticker` + `select` loop that reacts to `ctx.Done()` immediately. Targeted regression coverage was added in `internal/cli/daemon_commands_test.go` to verify prompt cancellation during warmup plus override wiring for attach snapshot and cancel timeout behavior.
- Verification: `go test ./internal/cli -run 'Test(DefaultAttachStartedCLIRunUICancelsOwnedRunOnLocalExit|NewAttachStartedCLIRunUIUsesConfiguredOwnedRunCancelTimeout|LoadUIAttachSnapshotWaitsForJobsWhenInitialSnapshotIsEmpty|LoadUIAttachSnapshotReturnsPromptlyWhenContextCanceledDuringWarmup|NewAttachCLIRunUIDisablesWarmupWhenConfiguredTimeoutIsZero)$' -count=1`; `make verify` (`0` lint issues, `DONE 2415 tests, 1 skipped`, build succeeded).
