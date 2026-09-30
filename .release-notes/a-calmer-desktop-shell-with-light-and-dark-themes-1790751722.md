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
- New windows open beside the one you are working in instead of floating on top of it.
- **Window › Arrange** offers Main and stack, Columns, Grid, and Balance sizes, and **Window › Move
  window to** sends a window to another desktop by name. `compozy layout arrange` and the
  `compozy__layout_arrange` tool accept the new `main_stack` arrangement, and an optional
  `--keep-frames` / `keep_frames` that arranges each named tab deck whole. Without it, every named
  window is still its own participant.
- Tabs look and behave like browser tabs, and window controls are quiet icons at the end of the head.
- An empty desktop says so and offers a way to start.
- Choose **Light**, **Dark**, or **System** in Settings › Appearance, or flip the theme from the dock.
  Dark stays the default. The choice is saved in this browser and applied before the first frame,
  and the terminal follows it.
- The interface uses Inter for text and Geist Mono for code, with higher-contrast secondary text in
  both themes.

### Migration

Three `[window_manager]` defaults change. Your saved layouts, profiles, and any value you set
explicitly in `config.toml` are kept as they are; only unset values pick up the new defaults.

| Setting                                            | Old default                   | New default    |
| -------------------------------------------------- | ----------------------------- | -------------- |
| `gaps.inner` / `top` / `right` / `bottom` / `left` | `8` / `8` / `10` / `8` / `10` | `0`            |
| `new_window_policy`                                | `floating`                    | `beside_focus` |
| `bindings.bottom_center`                           | `reserved`                    | `zoom`         |

To keep the previous behavior, set the old values explicitly:

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
