# BUG-20260713-loop-watch-poll-error-stuck: A watch-poll failure leaves an automated Loop running at generation zero

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-24 fire a filtered Trigger that starts a watch Loop
- **Scenarios:** TA-automation-crud-loop-target; LP-action-failure-detail
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** Trigger → Loop integrated replay

## Summary

Stopping the real task-role `system` session correctly fired the workspace Trigger once and delegated `reviews-watch` with `pr=2`. The Loop coordinator immediately logged a deterministic watch-poll error because the isolated workspace is not a git repository, but the Loop run stayed `Running`, `Generation 0`, `0s`, and `No generations yet` for more than two minutes. The UI showed no failure cause or recovery. Only an operator Stop transitioned it to `Failed`, still without the recorded cause.

## Reproduction

1. Enable a workspace Trigger on `session.stopped` filtered to `data.session_type=system`, targeting `reviews-watch` with `pr=2`.
2. Stop a matching real system session through the Web.
3. Open the delegated Loop run from Trigger history.
4. Let the watch poll fail in a workspace without git metadata and observe the run for at least two polling windows.

**Expected:** A deterministic coordinator/watch-poll failure terminalizes the run once with a typed bounded cause and recovery, or truthfully enters a retry/watch state with visible timing and diagnostics. It cannot remain falsely Running with no generation.
**Actual:** `looprun-56929015a03ab48d` stayed Running at generation 0 for more than two minutes. The daemon logged `scheduler.cycle.error` with the watch-poll failure immediately. Operator Stop finally marked it Failed but the UI still rendered no cause.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-trigger-system-stop-dispatch.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-trigger-delegated-looprun-after-window.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-trigger-loop-watch-poll-stuck-stopped.dom.txt`
- Daemon log line 52381: coordinator `run.loop.looprun-56929015a03ab48d.g1.coordinator` failed `loop watch poll` because `gh repo view` and `git remote get-url origin` ran outside a git repository.
- Trigger `trg-adaa781560301979`; automation run `run-69cff9e7ab61b043`; Loop run `looprun-56929015a03ab48d`.

## Fix

- **Root cause:** The coordinator lease-failure transaction terminalized the task run and cleared its lease, but it did not atomically settle the correlated Loop run or append a typed run-level failure event. A failure before generation one therefore left the Loop projection permanently Running with no generation-owned diagnostic.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** The canonical daemon scheduler/runtime integration suite owns the generation-zero watch-poll invariant against the real task manager, Loop coordinator, and SQLite store. Web run-event/stream/page suites own terminal replay, stream close, and typed failure projection before any generation exists.

## Verification

- Same-persona Trigger dispatch replay passed. Stopping the correlated real system session created exactly one additional delegated run, `looprun-4bc3d180d2edd5ba`. Its deterministic non-git watch-poll error automatically settled the run Failed at generation zero, rendered the bounded safe source cause and recovery with zero attempts, required no operator Stop, and did not create a duplicate run.
- **Fresh 2026-07-14 control:** Browser-created Trigger `qa-generation-zero-replay-0714-0153` matched stopped user session `sess-19c9b67078687781` and delegated exactly `looprun-a52b84b65ffba9b6`. The run settled Failed in 0s at generation/attempt zero, retained the typed cause and recovery after reload, exposed no Stop action, and emitted only running → failed. Exact-name Trigger deletion restored the catalog to zero; the probe session was also deleted. Evidence: `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/trigger-loop-generation-zero-replay-0714.dom.txt`.
