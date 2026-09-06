# BUG-20260906-inactivity-stop-presentation: verified inactivity stop reads as a one-second failure

- **Status:** fixed — real warning, clear and inactivity-stop re-walks passed
- **Impact:** Confusion · **Severity:** Major · **Priority:** P1
- **Persona:** Théo · **Scenario:** RT-visible-session-streaming
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

In the isolated lab, configure quiet_after=10s, stop_grace=30s and heartbeat=1s.
A real controlled ACP prompt emits initial text, new progress at25s and then blocks.
The daemon persists exactly two warning episodes, clears the first on progress, and
stops the process after the second grace. Public SessionPayload truthfully reports
stop_cause=inactivity, stop_reason=timeout, stop_detail=inactivity, verified=true,
escalated=true. The authored turn runs from13:54:19 to13:55:28.

Web instead shows Session failed — inactivity and Failed after1s. The terminal event
belongs to the established separate synthetic stop turn; lastSettledTurn restricts its
scan to that group and lets timeout failure beat the explicit session stop cause.
The SessionErrorNotice has the same misclassification. Evidence: quiet-events.json,
quiet-final-detail.json, quiet-stopped-status.json and quiet-second-warning-web.json/png
under the integrated lab evidence root. This attempt missed timed warning screenshots
because the probe initially read the outer response instead of session; that setup error
is separate from the independently reproduced final presentation defect.

Frontend repair must preserve distinct backend turn identities, use durable timestamps,
and retain actual provider-error behavior. No duration or quiet span may be guessed from
browser time. Final warning/clear/stop browser re-walk remains pending.

Verified at14:32 UTC through a fresh public prompt on the same real lab session
sess-b36e351dbe08b6f6, turn4b5522421e45fd48. The Web now reads "Stopped after1m9s ·
no work for41 seconds" and no session failure notice. Public resource records
inactivity, verified=true, escalated=true. Evidence: quiet-final-verified-{public,
transcript,web}.json and quiet-final-verified-web.png (root inspected). Reload
of the previous episode also read1m9s/no work42s. Canonical status and runtime
notice suites pass; the raw-shaped receipt input preserves true failure precedence.
