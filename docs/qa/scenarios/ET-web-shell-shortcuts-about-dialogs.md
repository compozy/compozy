---
id: ET-web-shell-shortcuts-about-dialogs
area: ET
title: Read the keyboard reference and installation identity from the shell
persona: Bruno
journey: J-operate-desktop-shell
expected: Help → Keyboard shortcuts opens a shell-scoped dialog listing Shell (including Global scope ⇧⌘G), Window, Layout, and Desktops sections from the live window-manager registry — every action present, an unbound action shown with an em dash rather than omitted, live config overrides reflected — with a footer that opens Settings → Layouts; Compozy → About Compozy opens a dialog showing only fields `/api/status` publishes (version, status, started, pid, HTTP host:port, socket, user home dir, config file) and degrades honestly while the status query is pending or failing; both dialogs are keyboard-reachable, scroll within a capped height, close on Esc, and return focus to the desktop.
entry_points: web desktop menubar Help menu; web desktop menubar Compozy menu
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/041-shell-shortcuts-about-dialogs
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-web-menubar-menu-set; ET-web-command-palette-shortcuts
---

story: As a builder, I can find every bound chord and identify exactly which daemon build my desktop is talking to, without the UI inventing values the runtime never publishes.

qa-impact: 2026-07-24 both dialogs are new. They replace the unfocusable raw `<div>` of shortcut
rows that used to render inside the Help dropdown (the menu opened with zero focusable children).
Flag only; the next QA cycle owns live retesting.

QA impact 2026-07-25 (deep-review remediation): the Layouts shortcut is now disabled when the live
window-manager command fence is unavailable. Flag only; the next QA cycle owns degraded-state
keyboard and pointer retesting.

QA 2026-07-29: The real desktop menubar opened both dialogs by keyboard. The shortcuts reference
listed all 27 live actions across four sections, including four em-dash unbound actions, and exposed
the enabled Layouts footer under the available command fence. About matched all eight `/api/status`
fields, invented no unpublished build metadata, retained the last snapshot during an induced poll
failure, closed on Escape, and returned focus to each owning menu trigger.

2026-08-12 qa-impact: Shell section now lists Global scope ⇧⌘G. Reset to untested.

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.
