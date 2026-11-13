---
id: ET-web-empty-desktop-card
area: ET
title: A new or emptied desktop opens one New tab window
persona: Bruno
journey: J-operate-desktop-shell
expected: A desktop is never empty: a new desktop created empty (Desktops Overview, or **New desktop** in a top-bar desktop dot's menu) and a desktop whose last tab closed or moved away both get one tab holding a **New tab** window — the crumb "New tab", the line "This tab is empty", the **Choose a destination** button, and "or press ⌘K" with the live palette chord; Choose a destination (or ⌘K) opens the palette scoped to that window and picking an app or route replaces the New tab window in place, keeping the tab; ⌘T opens the same New tab window as a new tab; a desktop whose tabs are all New tab placeholders counts as empty, so `compozy desktop delete` removes it without `--destination` and drops the placeholders. No session composer, no card, no modal.
entry_points: web desk on a new or emptied desktop; New tab window Choose a destination; ⌘K; ⌘T; compozy desktop delete
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-10-20-empty-desktop-composer/runtime-comparison.png; docs/qa/evidence/2026-10-20-empty-desktop-composer/session-delivered.png
last_report: docs/qa/reports/2026-10-20-empty-desktop-composer.md
overlaps: ET-web-desktop-shell-lifecycle; RT-desktop-pager-overview; ET-web-dock-contextual-session-launch
---

Added 2026-09-30 for the shell rail (VC-10 new empty desktop); reworked the same day for polish P7 (ChatGPT-style start: question + session composer, no "{desktop name} is empty" card). Walk: open a new desktop from the overview, type a prompt without clicking first and confirm it lands in the composer, use the desktop-switch chords from the empty desk and confirm they still switch desktops, pick another agent and press Enter — the session opens on this desktop filling the panel and its first message is the prompt; stop the agent's provider (or use an agent that cannot start) and confirm the error toast with the words still in the composer; clear the project's default agent and confirm Enter opens the New session dialog, dismiss it and see the words return; turn on All projects and confirm the composer has no agent control, sending opens New session with the project picker, and no surface reads "Pick a project"; close and minimize windows to bring the surface back; repeat in light theme.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

QA impact 2026-10-19 (#825): a just-created session now lands in an existing Session window on the landing desktop (focused, else empty-state, else most recently focused; pinned and other-desktop windows skipped) instead of opening another window. The window choice is owned by ET-web-new-session-reuses-session-window; replay this scenario's create step with a Session window already open.

QA impact 2026-10-20 (#862): select a runtime before a session exists, change reasoning/Fast where advertised, reload to confirm text/runtime persistence, attach a file and send once. Confirm the resulting session owns the selected runtime and file, then compare both composers. Repeat at a narrow viewport and retain the All projects composer canary (no "Pick a project" anywhere). Local file selections remain browser-memory drafts until a session owns their upload.

Regression follow-up (#862, first-message upload): hold an attachment upload, remove the file, then close/reopen both before upload cleanup settles and after manually sending the remaining text. The removed file must never return; the text must not send again. Also manually submit a first-message composer while its upload is pending and verify reopening cannot replay the handoff.

QA impact 2026-11-13 (window-tabs-split S9): a new desktop can also come from **New desktop** in a
top-bar desktop dot's menu; walk the empty-desktop start from there once. A desktop whose last tab
is dragged to another desktop keeps a New Tab window rather than going empty.

qa-impact: 2026-11-13 window-tabs-split S10. The empty-desktop composer ("What should we work on?") was removed with tabs and splits: an empty desktop now gets one New tab window (`internal/windowmanager/empty_desktop.go`, `web/src/systems/os/apps/new-tab/new-tab-window.tsx`). Earlier passes and evidence above describe the composer and are history only. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build. Create a desktop from Desktops Overview and from a top-bar dot's menu: each shows one tab with the New tab window. Click Choose a destination, pick Tasks: the same tab now shows Tasks. Close the last tab of a desktop: a New tab window appears. Drag the only real tab of desktop 2 to desktop 1: desktop 2 keeps a New tab window. With desktop 2 holding only New tab placeholders, run `compozy desktop delete --workspace <id> --revision <r> --id <desktop-2>` without `--destination`: it succeeds and no placeholder moves to another desktop.
