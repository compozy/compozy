---
id: ET-web-session-thread-full-bleed
area: ET
title: Session transcript and composer span the full window width
persona: Bruno
journey: J-12
expected: In an open Session window, the message transcript rail and the composer textbox share only horizontal inset padding (px-4/px-8) and span the full window content width; neither surface is capped by a centered max-width reading column; resizing the window wider keeps both surfaces edge-to-edge with the window body (minus inset).
entry_points: web desktop Session window; ThreadContentRail; session composer
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-sidebar-parent-20260806-212647-734931-lab/qa-artifacts/qa/journey-log.jsonl
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-web-dock-default-window-size; RT-session-lifecycle-affordances
---

story: As a builder I use a wide Session window and the chat transcript plus composer fill the available width instead of sitting in a narrow centered column.

qa-impact: Removed ThreadContentRail max-width/centering so session chat is full-bleed (2026-07-22). Flag only; the next QA cycle owns live retesting.
2026-08-06 session-sidebar impact flag: the session window gains a left sessions rail (closed by default, PanelLeft topbar toggle); full-bleed remains the default state but the layout is now conditional. Reset to untested for the next QA cycle.

2026-08-06 re-walked live: session window opens with the sessions rail closed (width 0) and the transcript+composer full-bleed; closing the rail after use restores full width. Evidence: lab journey-log.jsonl. Verdict: pass.
