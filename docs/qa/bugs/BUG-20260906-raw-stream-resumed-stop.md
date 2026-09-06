# BUG-20260906-raw-stream-resumed-stop: Raw replay closes on an older stopped episode

- **Status:** fixed — real SQLite/HTTP integration passed
- **Impact:** Stale-State
- **Severity:** Major · **Priority:** P1
- **Scenarios:** RT-visible-session-streaming
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

A raw SSE reconnect to a resumed active session replays the previous session_stopped event and previously returned immediately, dropping new live events. The raw stream now refreshes the current manager status after durable catch-up and closes only a currently stopped session. When a terminal frame was already replayed, it does not send a duplicate.

The canonical TestHTTPSessionStreamReconnectsWithLastEventID integration creates and prompts a real session, stops and resumes it, opens raw SSE, prompts again, and compares every replay/live frame ID to the public durable events. Before the repair, only7frames arrived versus12expected. Afterward the HTTP selection passes15.011s; core unit and tagged integration selections pass too. Evidence: `.cache/sessions-final-review-raw-resume-{red,green}.log`, `.cache/sessions-final-review-raw-core-green.log`, `.cache/sessions-final-review-core-integration-green.log`. This is an actual HTTP/SQLite run, separate from earlier browser transcript SSE evidence.
