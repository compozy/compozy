---
id: ET-terminal-browser-lifecycle
area: ET
title: Use persistent terminals from the browser
persona: Marina
journey: J-operate-integrated-terminal
expected: Clicking the Terminal dock item lands in a working terminal directly; New terminal opens a second one as an OS window tab; reloading preserves both; closing the window never ends a running terminal, and reopening adopts the newest running one; closing an already-ended terminal is a quiet no-op, never an error toast.
entry_points: Web dock Terminal app; /terminal; /terminal/{terminal_id}
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-08-26-integrated-terminal.md
overlaps:
---

Flagged by integrated-terminal task 06; reset 2026-08-31 by the window-native
UX rework (internal tab strip removed; id-less route resolves itself; close is
idempotent). The prior pass predates that surface.

Walk:

1. Click the Terminal dock item in a project with no terminals; a working terminal opens with no launcher or empty-state click in between.
2. Use the head's New terminal; a second terminal joins the frame as an OS window tab; switch between both deck tabs and reload the browser.
3. Close the Terminal window while one command is still running; confirm via `compozy terminal list` both sessions keep running; click the dock item and confirm it reattaches to the newest running session.
4. Let a terminal end, then close it again from the head's overflow; confirm the recorded exit is reported with no error toast, and the exit bar owns the story with no "Reconnecting…" line.
5. Confirm the route, window title, and dock badge remain truthful throughout.
