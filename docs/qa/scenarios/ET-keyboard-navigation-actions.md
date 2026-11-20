---
id: ET-keyboard-navigation-actions
area: ET
title: Navigate sessions workspaces desktops and attention by keyboard
persona: Bruno
journey: J-operate-desktop-shell
expected: Live keymap actions cycle visible sessions and workspaces with wrap, focus the newest needs-you session across workspaces, switch and create desktops, toggle the session sidebar, and no-op calmly when no target exists.
entry_points: web desktop keyboard; Keyboard shortcuts reference; Settings › Shortcuts
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; .compozy/tasks/herdr-parity/evidence/visual/task_05
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: ET-web-command-palette-shortcuts; RT-workspace-overview-command-tab
---

Flagged 2026-08-16 for the Herdr parity QA tail. Walk the visible session order in Recent, All,
and All workspaces scopes, including collapsed groups and cross-workspace attention landing.

QA 2026-08-16 Herdr parity: The full Web E2E, daemon settings contract suites, and inspected visual bundles covered editable shortcuts, array/range persistence, blocked and shadowed diagnostics, Terminal preset preview/apply/revert, live cheatsheet freshness, and editable-context routing.

qa-impact: 2026-11-18 keyboard-first navigation (#1826). Shortcuts moved from Settings › Layouts to
their own Settings › Shortcuts page (shortcut table, aliases, conflicts and global hotkeys); walk the
steps above there. An old `/settings/layouts?command=<id>` link opens Settings › Shortcuts on that row.

qa-impact: 2026-11-18 keyboard-first defaults (#1826). With an open session rail, Previous and Next
session (Control+Option+↑/↓, Control+Option+K/J, Control+Shift+↑/↓) walk project, folder and group
headers and session rows in screen order and wrap at both ends; a header takes visible focus and
→ / ← expand and collapse it; a session row opens its session. Option+H/J/K/L move focus between
split windows, Option+O returns to the last window, Option+; and Option+Shift+; split right and
down into a New tab pane that raises the destination picker, and Control+Option+H/L step tabs.
Picking a destination in that picker opens it in the split pane, which keeps its place; it never
opens a separate tab. While the picker is open, Option+; and other window chords do nothing behind
it. A header reached with Control+Option+↓ after a mouse click shows the focus ring. With a
full-height window beside a stacked pair (Split right, then Split down on the right), Option+J
from the top pane focuses the pane under it and Option+K comes back; neither jumps to the tall
window, which only Option+H reaches.

qa-impact: 2026-11-20 composer focus on session open (#1868). Clicking a session row, pressing
Enter on it, opening a session from the command palette or a notification, or following a deep
link puts the caret in that session's prompt composer, so typing goes straight into the prompt
(only when the session can take a prompt). Walking the rail with Previous/Next session, or
stepping a board drawer, does not: focus stays on the row (or in the composer if it was already
there). A focused dialog, find bar, decision or clarify dock, permission prompt, inline message
edit, or a terminal or Files window keeps focus. Clicking anywhere right after opening also wins.
