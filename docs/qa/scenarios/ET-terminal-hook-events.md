---
id: ET-terminal-hook-events
area: ET
title: Observe every terminal lifecycle hook with the owning scope
persona: Dora
journey: J-supervise-agent-terminal
expected: Every supported terminal transition emits exactly one asynchronous terminal hook carrying the terminal, workspace, profile, actor, run, and command fields owned by that event, while unrelated or reported-only output emits none.
entry_points: terminal.opened; terminal.closed; terminal.lease_changed; terminal.command_started; terminal.command_finished; terminal.input_requested; terminal.input_provided; terminal.recording_started; terminal.recording_stopped; terminal.subscriber_evicted; terminal.limit_rejected
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/test-e2e-runtime-after-fix.log; docs/qa/reports/2026-08-26-integrated-terminal.md
last_report: docs/qa/reports/2026-08-26-integrated-terminal.md
overlaps: ET-agent-reported-terminal; ET-terminal-agent-handoff-input
---

Flagged by integrated-terminal task 09. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Open and close a terminal; observe exactly one `terminal.opened` and one `terminal.closed` event with the same workspace, profile, terminal, actor, and run scope.
2. Claim, take, and yield control; observe ordered `terminal.lease_changed` events whose previous and next controller fields match the public terminal state.
3. Start and finish one successful and one failed command; observe paired `terminal.command_started` and `terminal.command_finished` events with stable command IDs and truthful outcomes.
4. Request, answer, and reject hidden input; observe `terminal.input_requested` and `terminal.input_provided` without the secret value.
5. Start and stop recording; observe `terminal.recording_started` and `terminal.recording_stopped` with the retained recording identity.
6. Overflow one bounded subscriber and reach the configured terminal limit; observe `terminal.subscriber_evicted` and `terminal.limit_rejected` with typed reasons, then confirm reported-only agent output emits none of these events.
