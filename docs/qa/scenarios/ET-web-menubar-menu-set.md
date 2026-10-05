---
id: ET-web-menubar-menu-set
area: ET
title: Operate every desktop surface from the static menubar
persona: Bruno
journey: J-operate-desktop-shell
expected: The CompozyOS mark is one `role="menubar"`. The Global scope globe sits between the mark and the workspace chip, outside `role="menubar"`. The workspace chip is a second `role="menubar"`. The static Session · Go · Window · Help set is a third `role="menubar"`; ←/→ traverse each menubar, Home/End jump to the ends, ↓ enters a menu, → opens a submenu, Esc closes, and hovering a sibling switches the open menu. Compact viewports (<960px) hide the app menus but keep mark · globe · chip leading. CompozyOS opens About/Settings/Appearance/Layouts, Session creates a session or agent and opens the catalog, Go mirrors the dock groups plus the palette and Workspaces, Window carries the everyday window actions in the shell-rail order — an Arrange group (Main and stack, Columns, Grid, Balance sizes), a Move window to submenu naming every other desktop, Zoom, Minimize, Floating, Move & resize, All desktops, Close — with real chords while directional focus, tab merge/detach, layout undo/redo, and desktop stepping stay palette/keyboard commands, and Help reaches the shortcut reference, docs, protocol, changelog, issues, and support; every item is disabled — never hidden — when its runtime predicate fails, and no chord glyph renders unless the registry actually binds it.
entry_points: web desktop menubar; keyboard traversal from the menubar; <960px compact viewport
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-desktop-shell-lifecycle; ET-web-command-palette-shortcuts; ET-web-window-routing-lifecycle
---

story: As a builder, I can reach and understand every desktop capability from a menu bar that behaves like an OS menu bar and never offers a command the daemon cannot run.

2026-09-29 qa-impact (shell-rail D3): the Window menu leads with a labelled Arrange group (Main and
stack, Columns, Grid, Balance sizes — registry titles), adds a Move window to submenu listing the
other desktops by name (registry slots `window.move_to_desktop.1..9`), and `desktop.overview` is now
titled All desktops. Flag; the next QA cycle owns live retesting.

qa-impact: 2026-07-24 the menubar moved onto the `@compozy/ui` `Menubar` primitive and grew from
`Session · View · Help` (one Session item, six View items) to the static five-menu set. The Compozy
mark stopped opening the Dashboard and became the system menu; the workspace chip became a
menubar item; the Window menu now carries minimize/zoom/toggle-floating, Move & resize, Arrange
(incl. Balance), Focus, Undo/Redo layout, desktop switching, Desktops overview, and Close window,
all sharing the ⌘K palette's `useOsWindowCommands` predicates; below 960px the four app menus are
not rendered and the palette carries every action. Flag only; the next QA cycle owns live
retesting.

QA impact 2026-07-26: the visible system mark and menu label hard-cut to Compozy. Status remains
untested; the next browser QA cycle owns live retesting.

2026-08-12 qa-impact: the Global scope Switch sits outside `role="menubar"`; compact chrome keeps chip + Switch after app menus hide. Reset to untested.

2026-08-13 qa-impact: the Global scope globe moved between the Compozy mark and the workspace chip. Mark and chip are now separate `role="menubar"`s so the globe is not a menu item. Compact chrome keeps mark · globe · chip after app menus hide. Reset to untested.

2026-08-13 qa-impact: the Global scope Switch moved from after the identity cluster into the trailing chrome (before the bell). Compact chrome still keeps chip + Switch after app menus hide. Flag; the next QA cycle owns live retesting.

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

2026-09-28 qa-impact (ui-normie-pass/os-shell): the Window menu dropped its Focus submenu, Merge all/Detach tab, Undo/Redo layout, and Previous/Next desktop items (still reachable through ⌘K and their chords). The trailing cluster is calmer: the update offer is a ghost icon with an accent dot, ⌘K and Settings use styled tooltips, and a worktree fallback shows as a warning dot on the project chip with its sentence atop the project menu. "Add workspace…" reads "Add project…". Reset to untested.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
