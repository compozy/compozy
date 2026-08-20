---
id: ET-web-command-palette-shortcuts
area: ET
title: Open desktop apps, sessions, and actions from the keyboard
persona: Bruno
journey: J-operate-desktop-shell
expected: ⌘/Ctrl+K opens one global palette over any desktop or composer and filters real apps, sessions, workspaces, and actions; Enter performs the selected action; ⌘/Ctrl+J remains scoped to the session runtime picker; ⌘/Ctrl+N, ⇧⌘/Ctrl+S, ⌘/Ctrl+W, ⌘/Ctrl+M, ⇧⌘/Ctrl+G (Global scope), and Escape perform the documented shell actions with one-layer overlay unwinding. The palette lists "Turn on Global scope" / "Turn off Global scope" and "Switch to {name}" notes that switching a project turns Global off. Workspace rows never include `$HOME`.
entry_points: web desktop keyboard; command palette; session composer; menubar Help; Keyboard shortcuts dialog
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/scope-global.png; docs/qa/reports/2026-08-16-herdr-parity.md; .compozy/tasks/herdr-parity/evidence/visual/task_05
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: ET-web-desktop-shell-lifecycle; ET-web-window-routing-lifecycle; ET-palette-nested-views; ET-palette-sessions-view-switch
---

story: As a keyboard operator, I can discover and execute every global desktop action without losing the session runtime shortcut or trapping focus in stale overlays.

qa-impact: OS Shell Task 04 moved ⌘/Ctrl+K ownership to the global palette, rebound the runtime selector, and added the shell shortcut set. 2026-07-22 — Toggle sessions opens the global Sessions modal overlay instead of a persisted rail. 2026-07-24 — the palette's window/layout/desktop surface moved into the shared `useOsWindowCommands` model that the menubar's Window menu also consumes (same coordinator dispatch, same predicates), and the Help menu's raw shortcut list became a real dialog (`ET-web-shell-shortcuts-about-dialogs`). No chord changed. Flag only; the next QA cycle owns live retesting.

2026-08-12 qa-impact: ⇧⌘G toggles Global scope; palette actions turn Global on/off. Reset to untested.

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

2026-08-13 re-walk: the live palette exposed "Turn off Global scope" and "Switch to tmp turns Global scope off"; executing the workspace action closed the overlay, restored `tmp`, and refresh preserved the destination.

2026-08-16 qa-impact: Command palette and New session moved into the daemon-owned registry while
remaining available inside inputs; new keyboard navigation actions share that registry. Reset for
the Herdr parity QA tail.

2026-08-16 qa-impact: The palette root gained a Views group and a keyboard-hint footer; nested view
behaviour and the Sessions view are walked by `ET-palette-nested-views` and
`ET-palette-sessions-view-switch`. Already `untested`, so no further reset — this walk still owns
the flat root behaviour above.

QA 2026-08-16 Herdr parity: The full Web E2E, daemon settings contract suites, and inspected visual bundles covered editable shortcuts, array/range persistence, blocked and shadowed diagnostics, Terminal preset preview/apply/revert, live cheatsheet freshness, and editable-context routing.

2026-08-19 qa-impact: Shortcut settings now cover every registry command, add workspace-scoped aliases,
and apply rebinds live through the daemon-owned keymap. Reset to `untested`; task 12 owns the isolated
persona walk and visual-contract evidence.

2026-08-20 qa-impact: Palette fallback and live Palette settings now share the same root assembly.
Keep this scenario `untested`; task 12 owns the keyboard and cross-surface re-walk.
