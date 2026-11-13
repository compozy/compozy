---
id: MS-window-shortcut-retired-binding
area: MS
title: A shortcut bound to a retired window action loads, does nothing and can be removed
persona: Bruno
journey: J-administer-window-manager
expected: A stored binding to window.minimize, window.zoom, window.toggle_floating, window.tile.top-left|top-right|bottom-left|bottom-right, window.merge_all or window.tab.detach loads without error; pressing its chord in the shell does nothing; Settings › Layouts › Shortcuts lists it as its own row under its old name (for example "Zoom window" with `window.zoom`) with the "retired" flag and the hint "This action was retired with tabs and splits, so the shortcut does nothing. Remove it.", its chord button and add-alternate button disabled; its reset button, named "Remove the retired shortcut for …", deletes the binding from the stored config and the row disappears; the chord is free for another action; no retired action appears in the palette or the keyboard shortcut reference.
entry_points: Settings › Layouts › Shortcuts; user or workspace config.toml [window_manager.shortcuts]; compozy config get window_manager.shortcuts; web desktop keyboard; Help › Keyboard shortcuts
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-layout-editor-shortcut-recorder; MS-window-shortcut-arrays-ranges; ET-live-shortcut-cheatsheet
---

story: As an operator who bound keys to minimize, zoom or corner tiling, I can see that those bindings no longer do anything and clear them in one click.

scope: US-018 EC-1 of spec `window-tabs-split`. The list of retired IDs is mirrored in the web (`lib/window-manager-retired-shortcuts.ts`) and the daemon (`internal/windowmanager/retired_shortcuts.go`); both are removed in v0.8.0.

qa-impact: 2026-11-13 window-tabs-split S10. New scenario. Walk: preconditions — an isolated lab
with the daemon running, a project with two tabs, no shortcut overrides. 1) In the user
config.toml add under `[window_manager.shortcuts]`: `"window.zoom" = "control+alt+KeyZ"`,
`"window.minimize" = "meta+KeyM"`, `"window.tile.bottom-right" = "control+alt+shift+KeyK"` and
`"window.tab.detach" = ["control+alt+KeyD", "alt+KeyD"]`; run `compozy config reload`: it succeeds.
2) In the shell, press each chord with a window focused: no window minimizes, zooms, moves or
detaches, and no error toast appears. 3) Open Settings › Layouts › Shortcuts in All projects (user scope): rows "Zoom window",
"Minimize window", "Tile to bottom right" and "Detach tab" each show their stored chords, the
"retired" flag and the hint; clicking a chord does not start recording; the add-alternate button
is hidden. 4) Record `control+alt+KeyZ` for "Move to new tab": accepted without a conflict. 5)
Press the reset button on "Zoom window" (screen reader name "Remove the retired shortcut for Zoom
window"): the row disappears and `compozy config get window_manager.shortcuts -o json` no longer
has `window.zoom`; repeat for the other three. 6) Open the command palette and Help › Keyboard
shortcuts: neither lists minimize, zoom, float, a corner tile, merge all or detach. 7) Repeat step 1
in the workspace config and confirm the rows appear when Settings shows the project scope.

qa-impact: 2026-11-13 shortcut scope isolation. With user-scope bindings for a current command
(for example `session.new`) and a retired command (for example `window.zoom`), select a project
whose config has no shortcut table. The project editor lists only project-owned overrides and
retired rows; inherited active bindings remain in the effective keymap. Record a project shortcut:
the project file contains only that binding, with no user shortcuts, global-shortcut table,
aliases or inherited behavior settings copied in. Repeat with existing project-owned current and
retired bindings: recording preserves them, and Remove deletes only the selected project binding.
The user config stays byte-for-byte unchanged; All projects still shows its retired bindings for
removal there.
