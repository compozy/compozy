---
id: RT-new-session-fast-feedback
area: RT
title: Start a new session with immediate truthful feedback
persona: Bruno
journey: J-17
expected: Clicking New session opens launch details without a first-message composer; Create gives visible feedback within 100 ms, creates one durable session and activates its owner workspace before navigation within 250 ms, then the destination composer accepts the separate first prompt and its runtime selection without duplicate creation.
entry_points: web agent detail New session; web Agents Start session
qa_status: pass
bug_ids: BUG-20260713-cursor-model-startup-contract; BUG-20260713-new-session-modal-lingers; BUG-20260713-first-prompt-optimistic-stuck; BUG-20260713-stop-generation-local-stuck; BUG-20260729-accepted-start-stop-identity-race; BUG-20260730-session-create-window-intent
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-live-create.json;/Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-native-default-unset.json;/Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-native-create.json;docs/qa/reports/2026-08-13-issue-389-cursor-model.md
last_report: docs/qa/reports/2026-08-13-issue-389-cursor-model.md
overlaps: RT-010
---

Capture click-to-feedback, click-to-navigation, and click-to-composer-ready
durations separately with Cursor/Grok 4.5 selected.

The fixed replay kept truthful pending feedback until live ACP confirmation,
then released the modal before destination navigation without a post-success
focus-trap delay. Session admission validates the provider and preserves the
exact model ID; the ACP/provider boundary owns any later model rejection.

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

QA impact 2026-07-29: stopping immediately after durable acceptance now waits for the event recorder
and serializes the terminal transition with the immutable launch-identity commit. Reset remains
`untested`; a fresh browser timing/failure-path replay owns the final verdict.

QA impact 2026-08-13: an explicit Cursor model now has to match a fresh ACP-advertised catalog value
before creation; an omitted Cursor model must still use Cursor's native default. Reset for a fresh
create walk.
