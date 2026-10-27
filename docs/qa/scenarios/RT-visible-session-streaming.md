---
id: RT-visible-session-streaming
area: RT
title: Keep every visible session stream live
persona: Théo
journey: J-13
expected: Two session windows visible side by side on the active desktop keep streaming when focus moves between them; minimizing, switching desktops, or hiding one behind an inactive stack suspends only that hidden window, which catches up when visible again.
entry_points: OS shell with two side-by-side session windows; web session live tail
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-09-06-sessions-stability.md
last_report: docs/qa/reports/2026-08-11-frontend-performance.md
overlaps: RT-023; RT-013
---

Added after the visible-window ownership rule changed from focus-only to actual OS-shell visibility.

QA impact 2026-08-11: document visibility, hidden-window resource ownership, and decision-key gating changed. Reset for a fresh targeted browser walk.

QA 2026-08-11: visible live windows kept their transports; minimizing the task window suspended only its stream, and restoring it reopened one cursor-based connection. A document background/foreground pass reconciled current state without reload or duplicate ownership.

QA impact 2026-09-06 sessions-stability task_06 (final walk owned by task_10): sever a
busy window's stream and verify the reconnect chip after 2s, retry count, catching-up
until the durable watermark, and a gapless/duplicate-free transcript against history.
Exhaust retries and use Try again; preserve an unsent draft behind the disconnected
send guard. Hide/restore a window and verify paused/live ownership. During prompt POST,
verify one bounded 1s session/queue/decision control poll, then cursor+fence handoff;
disconnect the response body and verify pending-ack identity plus immediate tail reopen.
This is a planning flag; prior evidence does not cover these new behaviors.

QA plan 2026-09-06: the integrated sessions-stability task10 re-walk is scoped by
`.compozy/tasks/sessions-stability/memory/qa.md`. Earlier evidence remains historical;
the changed queue/transcript/connection behavior has no current integrated verdict yet.

QA 2026-09-06 integrated verdict: Both visible windows received real ACP checkpoints while focus moved. Hiding only the neighbor removed its source; restore caught up from its cursor. Actual connection errors produced grace, counted retries and terminal Try again; the offline draft was retained and Queue answered Not sent. Unblock/Try again confirmed the unchanged head within500ms without another event. See two-windows-final-* and transport-fixed-* in the integrated report.

- Final review regression (2026-09-06): Raw resumed-session replay regression: BUG-20260906-raw-stream-resumed-stop; owning real HTTP/SQLite reconnect case passes and preserves all durable cursors.

- Final review log-stream regression (2026-09-06): BUG-20260906-empty-log-head-cursor is fixed. Real observer/SQLite/HTTP coverage proves a stream opened with no durable head delivers the first new event even when its timestamp predates the connection. Evidence: `.cache/sessions-final-gate-stream-integration-green.log`.

- CI stress re-walk (2026-09-06): BUG-20260906-stream-redaction-storm-timeout is fixed. The unchanged real daemon storm preserves all 50,000 chunks and one degraded slow watcher under a one-core limit in 54.72s. Exact redaction skips only assignment scans whose required separator is absent. The integrated report retains the failure/profile/pass and 800-case output-equivalence proof; earlier two-window/browser evidence remains current.

The same storm follow-up now also derives required provider-token prefixes without changing regexes or their order. The unchanged one-core real journey passes in 16.83s; the expanded output corpus retains 11,840 identical hashes. See the same report and defect record for the later CI failure that motivated this second measured correction.

The real TCP pressure fixture now sets its unchanged small receive buffer before connection negotiation. Linux socket measurements proved the prior post-connect setup could buffer the entire response without saturating the subscriber. The same 50,000-chunk journey and assertions pass in 16.05s after this setup correction; the report retains the measured boundary and CI failure.

QA impact 2026-10-17 issue 764: while a session's transcript stream holds its live edge, its
detail, by-id, usage, usage-turns, and prompt-queue reads drop from 5 s polling to a 30 s safety
read; stream frames and `session_usage_changed` still invalidate them as changes happen. A
background tab (window without live data) no longer polls its queue and rereads it on
activation. Lab walk: a CLI prompt to the focused session reached the UI 172 ms after the prompt
returned; activating a background tab reread its queue immediately.


QA impact 2026-10-17 (web performance): Permission and turn-outcome projections now reuse facts from immutable message content; subagent roster readers filter unrelated cache events. Re-walk streamed text, permission resolution, stopped/verified turns, and roster-owner detach/reconnect. No live-provider re-walk is claimed in this change.

Scope, measured results, and verification limits: [Web performance report](../reports/2026-10-17-web-performance.md). Historical scenario verdicts above are not new verification of this revision. Heavy local gates are deferred to CI by explicit user instruction.

QA impact 2026-10-17 (concurrent-session responsiveness): Keep two visible sessions
streaming while catalog and attention updates arrive in bursts. Detail and by-ID
reads must finish, then reread the final authoritative state without a cancellation
storm. Verify replacement, removal and replay of transcript entries, retaining a
current view while obsolete historical arrays are released. Disconnect the last
live stream owner and verify the existing fallback reads re-arm for workspace and
profile lenses without an immediate unsolicited fetch; reconnect and retain the
safety cadence. The existing catalog-stream, transcript-query, query-options and
live-tail suites own these boundaries. Current measurements and execution status
are recorded in the [responsiveness report](../reports/2026-10-17-responsiveness.md);
the earlier scenario passes are historical.


The 2026-10-17 responsiveness slice also replaces usage-notification event blob
reads with metadata reads. Usage frames retain their sequence, turn ID, kind and
absence of a transcript SSE ID; text-only pages still advance the internal usage
cursor. Check both wake and polling paths, including an event committed between
usage and transcript snapshots. See the same dated responsiveness report for
focused evidence and CI/runtime validation limits.


QA impact 2026-10-26 (session responsiveness): repeat this journey while many sessions
are active. Confirm that detail, parent/child titles and counts, board pages, exact
totals, and usage updates remain current across profiles and workspaces. Exercise
usage bursts and hide/restore the streaming window; unrelated transcript/Goal reads
must not refetch for usage-only signals. Current measurements and verification limits
are in the [responsiveness report](../reports/2026-10-26-session-responsiveness.md).
The final-source integrated re-walk is pending; earlier passes remain historical.
