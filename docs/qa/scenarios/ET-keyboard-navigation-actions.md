---
id: ET-keyboard-navigation-actions
area: ET
title: Navigate sessions workspaces desktops and attention by keyboard
persona: Bruno
journey: J-operate-desktop-shell
expected: Live keymap actions cycle visible sessions and workspaces with wrap, focus the newest needs-you session across workspaces, switch and create desktops, toggle the session sidebar, and no-op calmly when no target exists.
entry_points: web desktop keyboard; Keyboard shortcuts reference; Settings › Layouts › Shortcuts
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-command-palette-shortcuts; RT-workspace-overview-command-tab
---

Flagged 2026-08-16 for the Herdr parity QA tail. Walk the visible session order in Recent, All,
and All workspaces scopes, including collapsed groups and cross-workspace attention landing.
