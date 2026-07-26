---
id: ET-web-session-thread-full-bleed
area: ET
title: Session transcript and composer span the full window width
persona: Bruno
journey:
expected: In an open Session window, the message transcript rail and the composer textbox share only horizontal inset padding (px-4/px-8) and span the full window content width; neither surface is capped by a centered max-width reading column; resizing the window wider keeps both surfaces edge-to-edge with the window body (minus inset).
entry_points: web desktop Session window; ThreadContentRail; session composer
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-dock-default-window-size; RT-session-lifecycle-affordances
---

story: As a builder I use a wide Session window and the chat transcript plus composer fill the available width instead of sitting in a narrow centered column.

qa-impact: Removed ThreadContentRail max-width/centering so session chat is full-bleed (2026-07-22). Flag only; the next QA cycle owns live retesting.
