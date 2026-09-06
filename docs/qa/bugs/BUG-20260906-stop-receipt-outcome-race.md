# BUG-20260906-stop-receipt-outcome-race: Restart can change a verified stop outcome

- **Status:** fixed locally — one-core reproduction, full session race suite and actual subprocess integration pass; new-head CI pending
- **Impact:** Correctness · Recovery
- **Severity:** Major · **Priority:** P1
- **Scenario:** RT-session-native-stop
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

PR #557 run `34062900278`, Go shard 7, failed the existing recovery-at-ack-persistence case. The recovered outcome had zero elapsed time, differing from the outcome returned before restart. The same unchanged assertion reproduces locally with `GOMAXPROCS=1`.

The process watcher may verify exit and journal the terminal receipt before the stop driver's call returns. The old receipt path invented a cooperative, zero-duration fallback without retaining it; the driver later stored a separate outcome. The session now freezes the first verified observation under its lifecycle mutex. The receipt and stop caller share that value. If the watcher wins, its elapsed time derives from the recorded stop start, and the currently attempted escalation phase is retained before the driver action. A later driver acknowledgement cannot replace that verified result. The watcher can still finish independently of a delayed driver call.

Invariant: a verified stop preserves its cause, phase, escalation and elapsed time through retry and manager restart, with one terminal event/notification. Owner: session stop settlement. Canonical suite: `TestSharedSessionStopOperation`, including existing event/ack/state-persistence recovery cases. Its assertions and deadlines are unchanged; the existing stop/process-exit finalization and forced/killed-phase cases remain applicable.

Evidence: `.cache/sessions-final-stop-receipt-onecore-red.log` (unchanged test on old production), `.cache/sessions-final-stop-races-onecore-green.log` (30 repetitions of the focused selection), `.cache/sessions-final-stop-full-race.log` (136.495s), `.cache/sessions-final-stop-real-integration.log` (three real subprocess stop/crash/resume journeys, 6.012s), and `.cache/sessions-final-stop-windows-build.log`. Receipt JSON/schema, public routes, native tool IDs and workspace isolation are unchanged.
