# Idle-runtime stop deadline fix — handoff (Opus)

This is the bounded follow-up to finding F2 in `03_opus_initial.md`, done as Claude Opus 5.5 in direct mode. There was no commit or push, `~/.compozy` was not mutated, and no other worker's edit was touched. The controller owns final integration, `make gate`, docs, QA and the PR.

## Problem

Stopping a released or replaced agent ran `driver.Stop` under the 5 s lifecycle cleanup context: `stopReplacedRuntime` used `m.lifecycleCleanupContext()`, with `defaultLifecycleTimeout = 5s`.

The ACP driver, however, budgets 5 s of shutdown grace plus 3 s of drain, and forces the exit when its caller's deadline expires first.

As a result, a slow but healthy cooperative exit (which Codex frequently needs) was cut short:
- The process was force-killed at about 5 s.
- The release was recorded as failed, logged as `stop idle runtime …: context deadline exceeded`.
- The next rebind had to stop the process again.

## Change (current worktree lines)

- **`internal/acp/client_control.go:164`** — new `Driver.StopBudget()` returning `stopTimeout + defaultStopDrainTimeout`. `Stop` uses it at line 147, so the advertised budget and the enforced one cannot drift. Stop behaviour is unchanged.
- **`internal/session/manager_idle_release.go`:**
  - line 169: the optional consumer-side interface `AgentStopBudgeter`, the same pattern as `AgentKiller` and `AgentExitVerifier`.
  - line 174: `runtimeStopContext()` grants `max(defaultLifecycleTimeout, driver.StopBudget())`, detached from cancellation like the cleanup context. Drivers that don't advertise a budget keep the old 5 s bound.
  - The comment on `unverifiedProcess` now names the stop budget.
- **`internal/session/manager_prompt_runtime.go`, `stopReplacedRuntime` only (lines 281–302):**
  - The stop runs under `runtimeStopContext()`.
  - The agent-stopped hook gets its own `lifecycleCleanupContext()` instead of whatever is left of the stop deadline.
  - The controller's `materializeRootDelegationTools` hunk (around line 360) is untouched.
- **`internal/session/acp_driver_adapter.go:135`** — forwards `StopBudget()`. Line 24 adds `var _ AgentStopBudgeter = (*ACPDriverAdapter)(nil)`, so production wiring is checked at compile time. Production builds the manager with this adapter directly (`internal/daemon/session_manager_factory.go:68`).

**Why the fix stays bounded.**
- The budget is the driver's own contract, and the driver still force-terminates when its budget expires; this is not a new magic timeout.
- Deadline errors are still returned.
- On failure, process ownership is still kept until the exit is verified (`release.finish(err)` and `waitRuntimeRelease` are unchanged).
- Live runtime replacement and release-retry stops share the same helper.

**Not shipped, per the controller.** The provider and elapsed-time telemetry I briefly added is gone; logs and error text are as in baseline.

## Invariant, owner and suite

- **Invariant:** an idle release or runtime replacement lets the driver finish its bounded stop/drain protocol, keeps process ownership until the exit is verified, and allows a later native `session/load`.
- **Owner:** the `internal/session` Manager. The driver budget contract belongs to `internal/acp`.
- **Canonical suite:** `TestSupervisionReleasesIdleRuntime` in `internal/session/prompt_activity_test.go`.
  - New subtest at line 1324: `Should let the driver finish a stop that outlasts the cleanup deadline`.
  - Test support: the shared `fakeDriver` in `internal/session/manager_test_support_test.go` gains `stopBudget`, `StopBudget()` and `stopWithContextHook`, the same pattern as the existing `cancelWithContextHook`, plus a compile-time `AgentStopBudgeter` assertion.

**What the regression models.** A Codex-like agent needs 6 s to exit cooperatively under an advertised 8 s budget. Like the ACP driver, the fake forces the exit and returns `context.DeadlineExceeded` whenever the deadline it receives is shorter than that.

**What the subtest asserts:**
- Supervision completes without error.
- The granted deadline is at least the 6 s cooperative exit and no more than the 8 s budget, so the stop is neither truncated nor unbounded.
- Exactly one stop call.
- The verified exit clears the recorded subprocess identity.
- The next prompt rebinds with `ResumeSessionID` set to the original ACP session (native load).

## Evidence

- **Before the production repair** (new test and fake support only): `go test ./internal/session -run 'TestSupervisionReleasesIdleRuntime/Should_let_the_driver_finish…' -count=1` failed with
  `Supervise() error = supervise session sess-1: session: stop idle runtime for "sess-1": context deadline exceeded`.
  This is the same signature as the nine production log lines on 2026-10-22.
- **After the repair:** all 9 `TestSupervisionReleasesIdleRuntime` subtests pass, including the new one.
- **Focused race run:** `CGO_ENABLED=1 go test -race ./internal/session -run 'TestSupervisionReleasesIdleRuntime|TestPromptActivity|TestSupervision' -count=1` → ok (15.2 s).
- **ACP package:** `CGO_ENABLED=1 go test -race ./internal/acp -count=1` → ok (15.0 s).
- `go vet ./internal/session ./internal/acp` → ok.
- `GOOS=windows GOARCH=amd64 go build ./internal/acp ./internal/session` → ok.
- `gofmt -l` on the changed files → clean.
- **Test-convention checker:**
  - `manager_test_support_test.go`: OK.
  - `prompt_activity_test.go`: the same 7 INLINE-CASE findings as baseline, all in unrelated pre-existing tests. My subtest adds none.
- **One full `CGO_ENABLED=1 go test -race ./internal/session -count=1` run** (log at `/tmp/harness-opus-scripts/session_race.log`) failed in 3 tests outside this slice. I attributed them from their messages only, without rerunning:
  - `TestAgentPreStartHookOpenCodeLaunchOrigin` (2 subtests): the expected cwd/command differ only by the `/private/var` vs `/var` macOS temp-dir prefix. This is the pre-start hook path, not stop/release.
  - `TestReplyWatchQueueRetry/…UT-041_UT-049`: the wake text now carries the `type=agent_message` truncation hint from the controller's in-progress `reply_watch_delivery.go` edit.
  - `TestSubagentLifecycle/Should_finalize_once_and_retain_result_limit…`: subagent result-limit behaviour, which is in the controller's in-progress subagent edits.
  - None of the three goes through `stopReplacedRuntime`, the idle release, or ACP `Stop`. Per instruction, I ran no further broad suites; the final `make gate` owns coverage.

## Residual notes (not changed)

- Two sibling callers still give `driver.Stop` only the 5 s cleanup context: `rollbackActivation` (`internal/session/manager_helpers.go`) and `cleanupFailedStart` (`internal/session/manager_lifecycle.go`). Both are start-failure cleanup, outside this incident.
- The session stop ladder's forced phase (`stopForcedGrace`, 5 s) is shorter than the ACP budget, but the ladder escalates to a verified kill.
- Releases still run serially inside `Supervise`, which holds `supervisionMu`. A slow release can now hold the sweep for up to the driver budget (about 8 s, previously 5 s).
- No real-provider idle-release walk was run, because that would mutate `~/.compozy`. A real Codex idle release in an isolated QA lab is the remaining real-run evidence.


## Controller disposition

Accepted after source/adapter review and the real daemon late-reply integration. The reported
reply/subagent test failures were exact old recovery-hint expectations; the owning suites now
assert the filtered hint, actual reply target/turn and unchanged Unicode truncation bound. The
three affected session test roots pass with the canonical macOS temporary directory. The
OpenCode hook mismatch was the environment's `/var` alias versus the production canonical
`/private/var` path; no authorization/path check or test assertion was weakened.

No fresh live-Codex timing benchmark was performed. Isolated real-provider QA would not require
mutating the operator home; the controller used the existing disposable-daemon/ACP/MCP suites
for this delivery's real-run boundary. The full local gate and exact-head CI remain separate
requirements recorded in the PR. Worker findings and remaining limits above are preserved.
