---
id: RT-new-session-fast-feedback
area: RT
title: Start a new session with immediate truthful feedback
persona: Bruno
journey: J-17
expected: Clicking New session opens a first-message composer; Send gives visible feedback within 100 ms, atomically queues the prompt, navigates to one durable `starting` session within 250 ms, and surfaces a durable startup failure without duplicate creation; the queued prompt remains available for explicit resume and is never posted a second time by the web client.
entry_points: web agent detail New session; web Agents Start session
qa_status: untested
bug_ids: BUG-20260713-cursor-model-startup-contract, BUG-20260713-new-session-modal-lingers, BUG-20260713-first-prompt-optimistic-stuck, BUG-20260713-stop-generation-local-stuck
fix_status: fixed
retest_status: pending
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/new-session-modal-timing.json
last_report: docs/qa/reports/2026-07-13-automation-features.md
overlaps: RT-010
---

Capture click-to-feedback, click-to-navigation, and click-to-composer-ready
durations separately with Cursor/Grok 4.5 selected.

The fixed replay kept truthful pending feedback until live ACP confirmation,
then released the modal before destination navigation without a post-success
focus-trap delay. Canonical model selection and invalid-model preflight are
covered by the same live/session-manager correction.

Post-fix Goal acceptance on 2026-07-13 reopened the scenario: session
`sess-e74df4386f8d5a77` became visually usable after a 14.764-second create,
but its first prompt remained optimistic for more than 64 seconds without a
daemon prompt request or durable transcript entry.

A second fresh modal-to-session replay reproduced the zero-POST failure 37.190
seconds after Start. Its Stop action also left assistant-ui submitted; that
coupled local-cancellation defect is source-fixed but pending Browser retest.
The catalog-stream remediation removed the live handoff failure: fresh session
`sess-2a768148b6106dc3` held one global catalog stream and submitted its first
`/goal` exactly once, four milliseconds after the click. Stop generation also
recovered the local composer after one successful daemon cancel. The scenario
remains failed only because its separate session-start latency budget was not
retested or changed in this batch; both first-prompt and Stop bug rows are
verified.

QA impact 2026-07-22: session admission is now durable and asynchronous, the Web polls `starting`
at 500 ms, and startup failure has an explicit recovery pane. Reset to untested for a fresh timing
and failure-path replay.

QA impact 2026-07-25: first-message creation is now atomic: the `201 starting` session already owns the queued prompt, navigation remains immediate, and resume delivers a prompt retained across startup failure. Reset to untested; flag only.
