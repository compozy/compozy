---
id: ET-terminal-hook-events
area: ET
title: Observe every terminal lifecycle hook with the owning scope
persona: Dora
journey: J-supervise-agent-terminal
expected: Every supported terminal transition emits exactly one asynchronous terminal hook carrying the terminal, workspace, profile, actor, run, and command fields owned by that event, while agent-internal command output emits none.
entry_points: terminal.opened; terminal.closed; terminal.command_started; terminal.command_finished; terminal.input_requested; terminal.input_provided; terminal.recording_started; terminal.recording_stopped; terminal.subscriber_evicted; terminal.limit_rejected
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-terminal-shared-control-20260904-204013-041114-lab/qa-artifacts/qa/live-evidence.md; docs/qa/reports/2026-09-04-terminal-shared-control.md
last_report: docs/qa/reports/2026-09-04-terminal-shared-control.md
overlaps: ET-terminal-shared-control
---

QA impact 2026-09-04: `terminal.lease_changed` was deleted with exclusive ownership. Reset to verify
the ten supported lifecycle hooks and prove shared writes do not invent a replacement control event.

Flagged by integrated-terminal task 09. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Open and close a terminal; observe exactly one `terminal.opened` and one `terminal.closed` event with the same workspace, profile, terminal, actor, and run scope.
2. Attach multiple interactive participants, write, and resize; confirm no ownership-transition hook is emitted.
3. Start and finish one successful and one failed command; observe paired `terminal.command_started` and `terminal.command_finished` events with stable command IDs and truthful outcomes.
4. Request, answer, and reject hidden input; observe `terminal.input_requested` and `terminal.input_provided` without the secret value.
5. Start and stop recording; observe `terminal.recording_started` and `terminal.recording_stopped` with the retained recording identity.
6. Overflow one bounded subscriber and reach the configured terminal limit; observe `terminal.subscriber_evicted` and `terminal.limit_rejected` with typed reasons, then confirm agent-internal (non-terminal) command output emits no terminal hook events.

2026-09-04 targeted re-walk: passed for the changed catalog and transition contract. Runtime discovery
returned exactly the ten documented lifecycle events, with `terminal.lease_changed` absent. Multiple
shared writers, resize, detach, signal, reconnect, and close produced no replacement ownership event;
the unchanged event-payload guarantees remain covered by the canonical hook suites cited by the report.
