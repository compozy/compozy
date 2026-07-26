---
id: ET-web-menubar-menu-set
area: ET
title: Operate every desktop surface from the static menubar
persona: Bruno
journey:
expected: The menubar is one `role="menubar"` holding the AGH mark, the workspace chip, and the static Session · Go · Window · Help set; ←/→ traverse it, Home/End jump to the ends, ↓ enters a menu, → opens a submenu, Esc closes, and hovering a sibling switches the open menu; AGH opens About/Settings/Appearance/Layouts, Session creates a session or agent and opens the catalog, Go mirrors the dock groups plus the palette and Workspaces, Window carries the full window/layout/desktop command set with real chords, and Help reaches the shortcut reference, docs, protocol, changelog, issues, and support; every item is disabled — never hidden — when its runtime predicate fails, and no chord glyph renders unless the registry actually binds it.
entry_points: web desktop menubar; keyboard traversal from the menubar; <960px compact viewport
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-desktop-shell-lifecycle; ET-web-command-palette-shortcuts; ET-web-window-routing-lifecycle
---

story: As a builder, I can reach and understand every desktop capability from a menu bar that behaves like an OS menu bar and never offers a command the daemon cannot run.

qa-impact: 2026-07-24 the menubar moved onto the `@agh/ui` `Menubar` primitive and grew from
`Session · View · Help` (one Session item, six View items) to the static five-menu set. The AGH
mark stopped opening the Dashboard and became the system menu; the workspace chip became a
menubar item; the Window menu now carries minimize/zoom/toggle-floating, Move & resize, Arrange
(incl. Balance), Focus, Undo/Redo layout, desktop switching, Desktops overview, and Close window,
all sharing the ⌘K palette's `useOsWindowCommands` predicates; below 960px the four app menus are
not rendered and the palette carries every action. Flag only; the next QA cycle owns live
retesting.
