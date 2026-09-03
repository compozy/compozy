---
id: ET-web-command-palette-shortcuts
area: ET
title: Open desktop apps, sessions, and actions from the keyboard
persona: Bruno
journey: J-operate-desktop-shell
expected: ⌘/Ctrl+K opens one global palette over any desktop or composer and filters real apps, sessions, workspaces, and actions; Enter performs the selected action; ⌘/Ctrl+J remains scoped to the session runtime picker; ⌘/Ctrl+N, ⇧⌘/Ctrl+S, ⌘/Ctrl+W, ⌘/Ctrl+M, ⇧⌘/Ctrl+G (Global scope), and Escape perform the documented shell actions with one-layer overlay unwinding. The palette lists "Turn on Global scope" / "Turn off Global scope" and "Switch to {name}" notes that switching a project turns Global off. Workspace rows never include `$HOME`.
entry_points: web desktop keyboard; command palette; session composer; menubar Help; Keyboard shortcuts dialog; Settings > Layouts > Shortcuts (whole-registry table, source filter, alias column); [window_manager.shortcuts] + [cmd_palette.aliases] in config.toml
qa_status: pass
bug_ids: BUG-20260829-command-palette-portal-remains-mounted
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/integrated-terminal-profile-retest; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-profile-retest-20260829-172042-776889-lab/qa-artifacts/qa/qa-audit-report.md; docs/qa/reports/2026-08-16-herdr-parity.md
last_report: docs/qa/reports/2026-08-28-integrated-terminal-rebase.md
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

Walk (task_11 plan — flat root + whole-registry keyboard truth):

1. ⌘K opens one global palette over any desktop or composer; ⌘J stays scoped to the runtime
   picker; the documented shell chords (⌘N, ⇧⌘S, ⌘W, ⌘M, ⇧⌘G, Esc) perform their actions with
   one-layer overlay unwinding.
2. Settings > Shortcuts lists the entire registry — filter by source (Core areas / each
   extension); record a chord that conflicts — the block names the culprit; overwrite explicitly —
   the loser is unbound and flagged; reset-one and reset-all restore daemon defaults.
3. Set an alias in the Alias column — invalid grammar (whitespace, > 32 chars) is rejected inline
   with the rule; the saved alias renders as "Title (alias)" on palette rows and ranks first for
   its exact text.
4. Rebind a command — the new chord dispatches immediately and appears on palette rows, menubar
   items, and the cheatsheet without reload; the cheatsheet groups extension bindings by source.
5. Confirm workspace rows never include `$HOME` and Global-scope toggles keep their documented
   palette wording.

2026-08-29 qa-impact: The shared Dialog portal could remain mounted after a command executed, leaving
an invisible overlay above the focused destination window. The working-tree fix makes the portal's
lifetime follow the root open state and adds component and OS-shell regression coverage. This
scenario was reset until the isolated browser walk was recorded below.

2026-08-29 targeted re-walk: passed. A trusted Chromium action opened Terminal through the palette,
focused the destination, and found zero palette or overlay nodes after settlement. The existing broader
shortcut/settings evidence remains valid; this walk reverified the shared overlay lifecycle changed by
the remediation. The strict QA evidence audit passed and teardown was clean.

Expected evidence: screenshots of the source-filtered table, the named-conflict block and flagged
loser, the alias on a palette row, and the cheatsheet grouping; note the chord used for the live
rebind check.
