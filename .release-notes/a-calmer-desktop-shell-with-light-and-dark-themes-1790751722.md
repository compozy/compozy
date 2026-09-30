---
title: A calmer desktop shell with light and dark themes
type: highlight
---

The CompozyOS desktop is redesigned around flat chrome and edge-to-edge windows, and it now comes in
light and dark.

- The dock moves to a slim rail on the left. Its foot holds your profile, the theme toggle, and
  Settings. Below 960px it becomes a bottom tab bar with the same controls.
- The top bar runs the full width. Desktop dots and an **All desktops** button sit in its tray, next
  to the bell and the command palette.
- Windows tile edge to edge with no gaps. A single hairline separates them: drag it, or focus it and
  use the arrow keys (hold Shift for bigger steps), to resize the panes on both sides.
- New windows open as a tab in the window you are working in instead of floating on top of it; on
  an empty desktop they fill it. Option-click a rail icon to open it split beside your window,
  Shift-click to open it on a new desktop, or right-click it for every destination, including a
  floating window. `compozy window open --floating` and the `floating` input of
  `compozy__window_open` ask for a floating window explicitly.
- **Window › Arrange** offers Main and stack, Columns, Grid, and Balance sizes, and **Window › Move
  window to** sends a window to another desktop by name. `compozy layout arrange` and the
  `compozy__layout_arrange` tool accept the new `main_stack` arrangement, and an optional
  `--keep-frames` / `keep_frames` that arranges each named tab deck whole. Without it, every named
  window is still its own participant.
- Tabs look and behave like browser tabs, and window controls are quiet icons at the end of the head.
- An empty desktop asks "What should we work on?" above the session composer. Type a prompt and press
  Enter to start a session right there, with your project's default agent already picked.
- Sessions use the full width of their window again, tool calls read as quiet rows, and the list of
  sessions is open by default.
- Choose **Light**, **Dark**, or **System** in Settings › Appearance, or flip the theme from the dock.
  Dark stays the default. The choice is saved in this browser and applied before the first frame,
  and the terminal follows it.
- The interface uses Inter for text and Geist Mono for code, with higher-contrast secondary text in
  both themes.

### Migration

Three `[window_manager]` defaults change. Your saved layouts, profiles, and any value you set
explicitly in `config.toml` are kept as they are; only unset values pick up the new defaults.

| Setting                                            | Old default                   | New default |
| -------------------------------------------------- | ----------------------------- | ----------- |
| `gaps.inner` / `top` / `right` / `bottom` / `left` | `8` / `8` / `10` / `8` / `10` | `0`         |
| `new_window_policy`                                | `floating`                    | `tab`       |
| `bindings.bottom_center`                           | `reserved`                    | `zoom`      |

`new_window_policy` now accepts `tab` (the new default) and `beside_focus` (tile new windows beside
the focused one) alongside `floating`. To keep the previous behavior, set the old values explicitly:

```toml
[window_manager]
new_window_policy = "floating"

[window_manager.gaps]
inner = 8
top = 8
right = 10
bottom = 8
left = 10

[window_manager.bindings]
bottom_center = "reserved"
```

Or apply them from the CLI, for example
`compozy config set window_manager.new_window_policy floating`. Changes apply live. The theme is a
per-browser preference, not a `config.toml` key, so there is nothing to migrate for it.
