---
id: ET-terminal-window-native-flow
area: ET
title: Terminal windows are the only terminal tabs
persona: Marina
journey: J-operate-integrated-terminal
expected: The Terminal window shows exactly one terminal with no in-app tab strip; more terminals arrive as OS window tabs or windows; Journal and New terminal live in the window head; the first paint uses the full window width without a tab switch.
entry_points: Web dock Terminal app; window head New terminal and Journal; dock right-click Open in new window / Open as tab
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/e2e/__tests__/terminal.spec.ts E2E-002/E2E-007/E2E-009/E2E-018 + terminal-window-app suite UT-122; docs/qa/reports/2026-08-31-terminal-stabilization.md
last_report: docs/qa/reports/2026-08-31-terminal-stabilization.md
overlaps: ET-terminal-browser-lifecycle
---

Added 2026-08-31 by the window-native UX rework (stabilization prompt
`docs/prompts/20260831-1333_integrated-terminal-stabilization-ux-rework.md`).

Walk:

1. Open the Terminal app; confirm the window shows one terminal with no second tab strip under the window head, and the grid fills the settled window width on first paint.
2. From the head, open the Journal; confirm the head shows the Journal crumb with a back affordance, and back returns to the same terminal with its scroll intact.
3. Use the head's New terminal; confirm a second terminal joins the frame as an OS window tab and the deck is the only tab strip visible.
4. Use the dock's right-click Open in new window; confirm another terminal opens without touching the existing ones.
5. End a session from the head's overflow Close terminal; confirm the window stays put on the exit bar until you close it yourself.
