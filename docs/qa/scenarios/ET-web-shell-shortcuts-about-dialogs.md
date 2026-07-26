---
id: ET-web-shell-shortcuts-about-dialogs
area: ET
title: Read the keyboard reference and installation identity from the shell
persona: Bruno
journey:
expected: Help → Keyboard shortcuts opens a shell-scoped dialog listing Shell, Window, Layout, and Desktops sections from the live window-manager registry — every action present, an unbound action shown with an em dash rather than omitted, live config overrides reflected — with a footer that opens Settings → Layouts; AGH → About AGH opens a dialog showing only fields `/api/status` publishes (version, status, started, pid, HTTP host:port, socket, user home dir, config file) and degrades honestly while the status query is pending or failing; both dialogs are keyboard-reachable, scroll within a capped height, close on Esc, and return focus to the desktop.
entry_points: web desktop menubar Help menu; web desktop menubar AGH menu
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-menubar-menu-set; ET-web-command-palette-shortcuts
---

story: As a builder, I can find every bound chord and identify exactly which daemon build my desktop is talking to, without the UI inventing values the runtime never publishes.

qa-impact: 2026-07-24 both dialogs are new. They replace the unfocusable raw `<div>` of shortcut
rows that used to render inside the Help dropdown (the menu opened with zero focusable children).
Flag only; the next QA cycle owns live retesting.

QA impact 2026-07-25 (deep-review remediation): the Layouts shortcut is now disabled when the live
window-manager command fence is unavailable. Flag only; the next QA cycle owns degraded-state
keyboard and pointer retesting.
