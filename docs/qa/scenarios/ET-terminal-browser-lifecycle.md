---
id: ET-terminal-browser-lifecycle
area: ET
title: Use persistent terminals from the browser
persona: Marina
journey: J-operate-integrated-terminal
expected: Clicking the Terminal dock item lands in a working terminal directly; New terminal opens a second one as an OS window tab; reloading preserves both; closing the window never ends a running terminal, and reopening adopts the newest running one; closing an already-ended terminal is a quiet no-op, never an error toast.
entry_points: Web dock Terminal app; /terminal; /terminal/{terminal_id}
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/screenshots/bruno-two-terminals-after-reload.png; docs/qa/reports/2026-09-01-integrated-terminal-review-r2.md
last_report: docs/qa/reports/2026-09-01-integrated-terminal-review-r2.md
overlaps:
---

qa-impact: 2026-09-01 deep-review round 2 changed terminal route retargeting, reconnect settlement,
window close handling, and browser recovery ownership. Reset for a focused public-surface re-walk.

2026-09-01 re-walk: passed. Bruno opened two browser terminals, executed a real shell command,
reloaded with both tabs intact, closed the window without ending either process, reopened the newest
running terminal from the dock, and closed an already-ended terminal without an error notification.
Independent `compozy terminal get|list` reads matched the rendered state throughout.

Flagged by integrated-terminal task 06; reset 2026-08-31 by the window-native
UX rework (internal tab strip removed; id-less route resolves itself; close is
idempotent). The prior pass predates that surface.

Walk:

1. Click the Terminal dock item in a project with no terminals; a working terminal opens with no launcher or empty-state click in between.
2. Use the head's New terminal; a second terminal joins the frame as an OS window tab; switch between both deck tabs and reload the browser.
3. Close the Terminal window while one command is still running; confirm via `compozy terminal list` both sessions keep running; click the dock item and confirm it reattaches to the newest running session.
4. Let a terminal end, then close it again from the head's overflow; confirm the recorded exit is reported with no error toast, and the exit bar owns the story with no "Reconnecting…" line.
5. Confirm the route, window title, and dock badge remain truthful throughout.
