---
id: ET-web-empty-desktop-card
area: ET
title: An empty desktop asks what to work on and starts a session from the first message
persona: Bruno
journey: J-operate-desktop-shell
expected: A desktop with no visible windows shows, on the flat desk panel with no card around it, the question "What should we work on?" above the real session composer (placeholder "Describe a task or ask a question", an agent control preset to the project's default agent, the shared session runtime selector with provider, model, reasoning, Fast and advertised options, native attachment picker/drop/paste previews, and the round send control) and a quiet "Press ⌘K to open anything" line with the live palette chord; typing while nothing else has focus lands in the composer and the shell's chords (⌘K, the desktop-switch chords, ⌘E) keep working; Enter or Send starts a session with the chosen agent in the scope's worktree, keeps the words in the resting composer with "Starting a session…" until it exists, then opens it on this desktop as the one window filling the panel with the message, selected runtime, and attached files as its first prompt; a failed start shows the error toast and leaves the words and staged files editable; when no agent resolves the New session dialog opens with the prompt, and dismissing it leaves the words in the composer; in Global scope the composer rests with "Pick a project to start a session" and a Pick a project button that opens the workspace menu; the surface returns when the last window closes or is minimized and reads in both themes.
entry_points: web desk on a new or emptied desktop; empty-desktop composer (Enter / Send); agent control; Pick a project (Global); ⌘K
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-10-20-empty-desktop-composer/runtime-comparison.png; docs/qa/evidence/2026-10-20-empty-desktop-composer/session-delivered.png
last_report: docs/qa/reports/2026-10-20-empty-desktop-composer.md
overlaps: ET-web-desktop-shell-lifecycle; RT-desktop-pager-overview; ET-web-dock-contextual-session-launch
---

Added 2026-09-30 for the shell rail (VC-10 new empty desktop); reworked the same day for polish P7 (ChatGPT-style start: question + session composer, no "{desktop name} is empty" card). Walk: open a new desktop from the overview, type a prompt without clicking first and confirm it lands in the composer, use the desktop-switch chords from the empty desk and confirm they still switch desktops, pick another agent and press Enter — the session opens on this desktop filling the panel and its first message is the prompt; stop the agent's provider (or use an agent that cannot start) and confirm the error toast with the words still in the composer; clear the project's default agent and confirm Enter opens the New session dialog, dismiss it and see the words return; switch to Global scope and confirm the resting composer and Pick a project; close and minimize windows to bring the surface back; repeat in light theme.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

QA impact 2026-10-19 (#825): a just-created session now lands in an existing Session window on the landing desktop (focused, else empty-state, else most recently focused; pinned and other-desktop windows skipped) instead of opening another window. The window choice is owned by ET-web-new-session-reuses-session-window; replay this scenario's create step with a Session window already open.

QA impact 2026-10-20 (#862): select a runtime before a session exists, change reasoning/Fast where advertised, reload to confirm text/runtime persistence, attach a file and send once. Confirm the resulting session owns the selected runtime and file, then compare both composers. Repeat at a narrow viewport and retain the Global Pick a project canary. Local file selections remain browser-memory drafts until a session owns their upload.
