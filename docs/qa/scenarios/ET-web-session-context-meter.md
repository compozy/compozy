---
id: ET-web-session-context-meter
area: ET
title: Composer context meter follows agent reports
persona: Bruno
journey: J-14
expected: The composer reserves a context control after the environment selector. The usage read supplies its ring, percent or used-only label, and a stale cue; the tooltip never shows a turn id, clock, or `reported` chip. Unknown never reads zero percent; unavailable retains last values. There is no warning band, threshold tick, or compaction sentence, because CompozyOS has no compaction threshold; the context rail's meter section offers a Compact now button only when the session advertises a compaction command (a button, not a popover; walked in ET-session-compact-now).
entry_points: OS session window composer; session-context-agent acpmock fixture; SessionContextControl stories
qa_status: untested
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
3. Send `warning`: 88% keeps the normal fill bands — no warning tone, no `almost full` chip, no threshold tick, and no compaction sentence in the tooltip or the rail meter.
4. Verify unknown, first-read pending, catalog window, used-only, stale, stopped, over-capacity, and unavailable states. Catalog size carries no threshold sentence; over-capacity keeps raw values and caps the arc; unavailable keeps last values.
5. While an ordinary turn remains running, after a terminal agent compaction (recorded Claude frames), the previous reading clears and the meter renders the existing unknown treatment (dashed ring, "Context usage unknown", never zero percent) until the next usage report carrying a context reading restores it. Both the tooltip and the rail meter then read "Context compacted. Waiting for the agent's next usage report."; a session that never reported keeps "This agent hasn't reported context usage.". A refetch that returns the pre-compaction reading (a response that predates the compaction) must not bring the old numbers back, while a fresher report does. The update and native marker must appear before the turn ends: compaction snapshots and attribution emit `session_usage_changed` (the rail also re-reads when its own Compact now request settles).
6. Confirm a later query supersedes the observation by ledger sequence; equal-sequence attribution still refreshes, and an explicit clear/reset removes retained context.
7. On a session whose agent advertises `compact` or `compress`, the rail's meter section shows an enabled Compact now button below the meter when the session is idle and active; it is absent for an agent that advertises nothing and disabled while a turn streams (full walk in ET-session-compact-now step 8).

Final execution and paired reference/implementation captures belong to task_06 VC-01–05. No walk has run for this change.

QA 2026-09-12: session-context final feature pass; runtime, focused integration/browser and 43 visual-pair evidence are separated in the linked report. Unchanged lifecycle/roll-up behavior reuses the earlier owning evidence; this pass verifies the new Context surface and its coexistence.

2026-09-15 quiet-context pass: the tooltip dropped its provenance row (`reported` chip, `as of turn <id>`) and the rail meter dropped its clock/threshold line; a plain report shows numbers only, and the chip appears just for loading, unavailable, stale, near compaction, or estimated size. Steps 1 and 3 updated. Owning unit suite (`session-inspector.test.tsx`) and the focused browser E2E (`session context E2E-001`) re-verified against the acpmock fixture; the design boards under `docs/design/opendesign/session-context/` still draw the earlier meter line and are superseded on this point.

QA impact 2026-10-07 (memory removal): the `warning` state, the `almost full` chip, the threshold tick, and the "CompozyOS summarizes older messages at {threshold} full" sentence were removed. The 2026-09-15 quiet-context note about a threshold tick is historical. Stale verdict reset to untested; historical evidence preserved; no QA session ran. Update 2026-10-08: Compact now and the post-compaction meter sentence shipped in the Web (steps 5 and 7).
