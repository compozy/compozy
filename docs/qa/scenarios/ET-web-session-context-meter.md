---
id: ET-web-session-context-meter
area: ET
title: Composer context meter follows agent reports
persona: Bruno
journey: J-14
expected: The composer reserves a context control after the environment selector. The usage read supplies its ring, percent or used-only label, and a stale cue; the tooltip never shows a turn id, clock, or `reported` chip. Unknown never reads zero percent; unavailable retains last values. Only an agent-reported eligible threshold produces a warning and the tooltip compaction sentence.
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

1. Open a live session using `session_context_fixture.json`, send `reported`, and observe 35% and `89.7K / 256K` on hover and keyboard focus. The tooltip carries only those numbers: no `reported` chip and no `as of turn` line.
2. Activate the control by Enter and by click. Both open Context and press the existing topbar toggle. Verify narrow composer wrapping preserves Send and attachments.
3. Send `warning`: 88% uses warning tone and the tooltip says `Compaction runs at 85%`. The rail meter shows the `near compaction` chip and threshold tick without that sentence.
4. Verify unknown, first-read pending, catalog window, used-only, stale, stopped, over-capacity, and unavailable states. Catalog size has no compaction sentence; over-capacity keeps raw values and caps the arc; unavailable keeps last values.
5. Confirm a later query supersedes the observation by ledger sequence; equal-sequence attribution still refreshes, and an explicit clear/reset removes retained context.

Final execution and paired reference/implementation captures belong to task_06 VC-01–05. No walk has run for this change.

QA 2026-09-12: session-context final feature pass; runtime, focused integration/browser and 43 visual-pair evidence are separated in the linked report. Unchanged lifecycle/roll-up behavior reuses the earlier owning evidence; this pass verifies the new Context surface and its coexistence.

2026-09-15 quiet-context pass: the tooltip dropped its provenance row (`reported` chip, `as of turn <id>`) and the rail meter dropped its clock/threshold line; a plain report shows numbers only, and the chip appears just for loading, unavailable, stale, near compaction, or estimated size. Steps 1 and 3 updated. Owning unit suite (`session-inspector.test.tsx`) and the focused browser E2E (`session context E2E-001`) re-verified against the acpmock fixture; the design boards under `docs/design/opendesign/session-context/` still draw the earlier meter line and are superseded on this point.
