---
id: ET-web-session-context-meter
area: ET
title: Composer context meter follows agent reports
persona: Bruno
journey: J-14
expected: The composer reserves a context control after the environment selector. The usage read supplies its ring, percent or used-only label, freshness, and provenance. Unknown never reads zero percent; unavailable retains last values. Only an agent-reported eligible threshold produces a warning and compaction sentence.
entry_points: OS session window composer; session-context-agent acpmock fixture; SessionContextControl stories
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-09-12-session-context.md
last_report: docs/qa/reports/2026-09-12-session-context.md
overlaps: ET-web-session-context-sidebar; ET-web-session-inspector-toggle
---

1. Open a live session using `session_context_fixture.json`, send `reported`, and observe 35% and `89.7K / 256K` on hover and keyboard focus.
2. Activate the control by Enter and by click. Both open Context and press the existing topbar toggle. Verify narrow composer wrapping preserves Send and attachments.
3. Send `warning`: 88% uses warning tone and the tooltip says `Compaction runs at 85%`.
4. Verify unknown, first-read pending, catalog window, used-only, stale, stopped, over-capacity, and unavailable states. Catalog size has no compaction sentence; over-capacity keeps raw values and caps the arc; unavailable keeps last values.
5. Confirm a later query supersedes the observation by ledger sequence; equal-sequence attribution still refreshes, and an explicit clear/reset removes retained context.

Final execution and paired reference/implementation captures belong to task_06 VC-01–05. No walk has run for this change.

QA 2026-09-12: session-context final feature pass; runtime, focused integration/browser and 43 visual-pair evidence are separated in the linked report. Unchanged lifecycle/roll-up behavior reuses the earlier owning evidence; this pass verifies the new Context surface and its coexistence.
