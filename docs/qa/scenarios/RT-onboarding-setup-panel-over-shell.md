---
id: RT-onboarding-setup-panel-over-shell
area: RT
title: First-run setup renders over an inert desktop shell
persona: Lea
journey: J-19
expected: With onboarding incomplete the desktop chrome renders behind a blocking setup panel — wallpaper, a menu bar whose workspace slot reads "No workspace" with no sync-status pill, and a dimmed dormant dock — while the panel owns focus; Esc and outside-press do not dismiss it, ⌘K and ⌘N do nothing, nothing behind the scrim is clickable or tabbable, the runtime popover still opens above the panel, and finishing setup wakes the same desktop (menu bar names the first workspace) without a full reload.
entry_points: web `/_app/` first-run against a fresh `AGH_HOME`; `desktop-gate.tsx`, `desktop-shell.tsx`, `onboarding-setup-panel.tsx`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-004
---

story: As a first-run operator I want to see the workspace I am about to unlock while setup asks its two questions, so first run reads as the OS onboarding me rather than a different application.

Added 2026-07-24 with the onboarding shell-panel redesign. Covers the shell requirements the panel places on the chrome rather than the two setup steps themselves (RT-004 owns those): the chrome must render with zero workspaces and zero windows, suppress the layout-connection pill while unbound, mark itself `inert`, and unbind the global shortcut listener for as long as setup is open.

Checks that only a browser pass can confirm: focus containment across Tab/Shift+Tab, Esc and outside-press being inert, the runtime selector popover portaling above the panel, the panel resizing from the one-column runtime step to the two-pane workspace split, and the dock/menu-bar wake transition on completion.

QA impact 2026-07-25 (deep-review remediation): setup step navigation is now inert while a step is
busy. Flag only; the next QA cycle owns pointer and keyboard attempts during pending transitions.
