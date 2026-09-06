# BUG-20260906-cancel-exited-session-race: Cancel can reject a session whose process just exited

- **Status:** fixed locally — deterministic regression, full session race suite and real stop/resume integration pass; new-head CI pending
- **Impact:** Correctness
- **Severity:** Major · **Priority:** P1
- **Scenario:** RT-session-prompt-cancel
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

PR #557 run `34062900278`, Go shard 7, failed the existing process-exit cancellation case with `session: session is not active`. CancelPrompt had found the session, but the process watcher transitioned it to stopping before the turn-stop claim. Final removal could also race that second lookup. A known session without a cancelable turn must retain its idempotent `nothing-in-flight` result.

CancelPrompt now maps those two completion races to `nothing-in-flight`, alongside its existing no-active-prompt result. Initial lookup failures and context errors remain errors. The stronger existing test holds the finalization catalog write at the I/O boundary after the process exits, then calls CancelPrompt while that known session is stopping. It fails on the old production implementation and passes after the repair. It also retains the assertion that the driver receives no cancel call.

Invariant: canceling a known session after its process exits is an idempotent no-op. Owner: session cancellation. Canonical suite: `TestCancelPrompt` in `manager_prompt_contract_test.go`; no new test file or duplicate layer was added.

Evidence: `.cache/sessions-final-cancel-exit-red-clean.log` (old production via Go overlay), `.cache/sessions-final-cancel-exit-final-green.log`, `.cache/sessions-final-stop-full-race.log` (136.495s), and `.cache/sessions-final-stop-real-integration.log` (three actual subprocess stop/crash/resume journeys, 6.012s). The focused one-core race selection passes 30 repetitions in 34.882s. No retry or deadline was added to product behavior.
