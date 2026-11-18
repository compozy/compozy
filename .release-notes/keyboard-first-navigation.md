---
title: Drive the desktop shell from the keyboard
type: feature
---

The default keymap now covers everyday navigation without leaving the home row. Option+H/J/K/L move
focus between windows and Option+O returns to the last one. Option+; splits the focused window to
the right and Option+Shift+; splits it down; each split opens a New tab pane ready for you to pick
what it shows. Option+W closes a window and Option+T opens a tab. Control+Option+H/L step through
tabs, and Control+Option+↑/↓ or Control+Option+K/J step through sessions. The previous defaults stay
as alternates: Control+arrows, Control+Option+O, Control+Shift+↑/↓, and Control+Tab.

Previous and Next session now walk the open session list in the order you see it, project and
folder headers included. A header takes focus, → expands it and ← collapses it, and a session row
opens its session.

Shortcuts keep working while you type. A chord with Command, Control, or Option fires from a text
field, the composer, or a terminal. Plain keys and Shift-only chords such as `?` stay typing. Text
fields keep undo, redo, select all, clipboard, formatting, and caret moves. In a macOS terminal,
Control chords that are not bound reach the shell, including ⌃K, ⌃V, and ⌃0. On Linux and Windows,
a chord whose only modifier is Control stays with the shell, except the palette, paste, copy with a
selection, and zoom. Holding a bound chord runs it once instead of typing the character.

Shortcuts have their own Settings page, Settings › Shortcuts, with the keymap table, aliases,
conflicts, and global hotkeys. The Layouts page keeps window behavior and saved layouts.

`compozy window open --tiled --edge bottom`, the `insert_edge` field of `window.open` on HTTP/UDS,
and `compozy__window_open` choose which side of the focused window a tiled open lands on. The field
is optional and defaults to the right.

## Migration notes

- **Default chords.** Control+Option+↑/↓ now cycle sessions instead of moving a window to the top or
  bottom edge of its tab; `window.tile.top` and `window.tile.bottom` are unbound by default. Bind
  them again in Settings › Shortcuts or under `[window_manager.shortcuts]`.
- **Your bindings are kept.** `[window_manager.shortcuts]`, every command id, and your overrides are
  unchanged. A stored binding that uses a chord the new defaults also use keeps it; the default
  gives that chord up.
- **Terminal preset removed.** Its chords are now the defaults, so Settings no longer offers the
  preset. Values you applied earlier stay in your config and keep working.
- **Moved page.** Shortcuts moved from Settings › Layouts to Settings › Shortcuts
  (`/settings/shortcuts`). An old `/settings/layouts?command=<id>` link opens the Shortcuts page
  focused on that command until v0.9.0.
