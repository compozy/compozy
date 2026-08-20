---
id: RT-onboarding-setup-panel-over-shell
area: RT
title: First-run setup renders over an inert desktop shell
persona: Lea
journey: J-19
expected: With onboarding incomplete the desktop chrome renders behind a blocking setup panel — wallpaper, a menu bar whose workspace slot reads Global (`~`) with the Global switch on (locked until a project folder exists) and no sync-status pill, and a dimmed dormant dock — while the panel owns focus; Esc and outside-press do not dismiss it, ⌘K and ⌘N do nothing, nothing behind the scrim is clickable or tabbable, the runtime popover still opens above the panel, and finishing setup wakes the same desktop (menu bar stays Global (`~`) when no folder was added, or names the first project workspace when one was) without a full reload.
entry_points: web `/_app/` first-run against a fresh `COMPOZY_HOME`; `desktop-gate.tsx`, `desktop-shell.tsx`, `onboarding-setup-panel.tsx`
qa_status: skipped
bug_ids: BUG-20260820-global-home-deleted-onboarding
fix_status: fixed
retest_status: pending
fix_commits: e520f3fe
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa; docs/qa/reports/2026-08-20-ui-normies-retry.md
last_report: docs/qa/reports/2026-08-20-ui-normies-retry.md
overlaps: RT-004
---

2026-08-20 retry: the affected lab reproduced `BUG-20260820-global-home-deleted-onboarding` and the
production fix passed scoped checks. The fresh same-persona replay was skipped by explicit user
instruction, so no behavioral pass is claimed and retest remains pending.

story: As a first-run operator I want to see the workspace I am about to unlock while setup asks its two questions, so first run reads as the OS onboarding me rather than a different application.

Added 2026-07-24 with the onboarding shell-panel redesign. Covers the shell requirements the panel places on the chrome rather than the two setup steps themselves (RT-004 owns those): the chrome must render with zero workspaces and zero windows, suppress the layout-connection pill while unbound, mark itself `inert`, and unbind the global shortcut listener for as long as setup is open.

Checks that only a browser pass can confirm: focus containment across Tab/Shift+Tab, Esc and outside-press being inert, the runtime selector popover portaling above the panel, the panel resizing from the one-column runtime step to the two-pane workspace split, and the dock/menu-bar wake transition on completion.

QA impact 2026-07-25 (deep-review remediation): setup step navigation is now inert while a step is
busy. Flag only; the next QA cycle owns pointer and keyboard attempts during pending transitions.

2026-08-12 qa-impact: first-run menubar chip reads Global (`~`), not "No workspace". Reset to untested.

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

2026-08-20 qa-impact: reset by the normie-friendly UI foundation pass. The setup frame's trust line
changed from "The daemon runs on this machine — no account, no upload." to "CompozyOS runs on this
machine — no account, no upload." (`onboarding-setup-frame.tsx:75`), and the workspaces catalog error
now reads "Check that CompozyOS is running." This is the one screen where a first-time person meets
the product's vocabulary before anything else, so it is the highest-stakes place for the plain
register to hold.

The panel behaviors this file owns — focus containment, inert chrome, suppressed shortcuts, the
runtime popover portaling above, the wake transition — are unchanged by the pass. Re-walk them
alongside the copy, and read the whole panel at the new 15px baseline: the type lift reflows the
two-pane workspace split, and the shell is where a first run either fits or clips.
