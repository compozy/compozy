---
id: ET-window-retired-surfaces-deprecations
area: ET
title: Retired window surfaces answer with a deprecation until v0.8.0
persona: Ada
journey: J-agent-manage-window-tabs
expected: Each retired surface keeps answering and names its replacement: `compozy window float|zoom|resize|group` and `compozy layout frame-resize` exit 0 and print one stderr line starting "warning:" that names v0.8.0; the native tools `compozy__window_float`, `compozy__window_zoom`, `compozy__window_resize`, `compozy__layout_frame_resize` and `compozy__window_group` return `applied: false` (group: the windows become tabs after the target) with a `deprecations[]` entry `{code: "window_manager_surface_deprecated", surface, replacement, removal_release: "v0.8.0", message}`; the HTTP command ids `window.stack.reorder` and `window.stack.set_active` act as `window.tab.reorder` and `window.tab.activate` and carry a deprecation; `window close --minimize` leaves the window open; `window open --floating` opens a tab; the retired palette command IDs `window.minimize`, `window.zoom`, `window.toggle_floating`, `window.merge_all`, `window.tab.detach`, `window.resize`, `layout.frame_resize` and `window.stack.group` are absent from `compozy cmd-palette list` and ⌘K but still resolve through `compozy cmd-palette invoke` and `compozy__cmd_palette_invoke` as deprecated no-ops (`applied: false` with a `deprecations` entry; the CLI prints `warning: command "<id>" is deprecated; … Remove before v0.8.0.`); `window.move` with an empty or missing `placement` (HTTP commands route or `compozy__window_move`) moves the window to its own tab and carries the deprecation `window.move.placement=empty`; none of them change the revision when nothing applies.
entry_points: compozy window float|zoom|resize|group; compozy layout frame-resize; compozy__window_float; compozy__window_zoom; compozy__window_resize; compozy__layout_frame_resize; compozy__window_group; POST /api/workspaces/{id}/window-manager/commands window.stack.*; compozy window close --minimize; compozy window open --floating
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-window-manager-public-parity; ET-window-tab-agent-parity; MS-window-manager-retired-config-keys; MS-window-shortcut-retired-binding
---

story: An agent or script written for v0.6 keeps running after the upgrade and is told, on every call, what to use instead.

scope: The deprecation window for retired CLI verbs, native tools, command ids and flags. Config keys and shortcuts are MS-window-manager-retired-config-keys and MS-window-shortcut-retired-binding.

Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build (unique home, port and socket). Open two windows in separate tabs and record the revision. Run each retired CLI verb with valid arguments and `-o json`: exit 0, stdout `applied: false` (group: the windows moved as tabs after the target) and one `warning:` line on stderr naming the replacement and v0.8.0; the revision is unchanged for the no-ops. From an agent session call each retired native tool and confirm the `deprecations` entry fields. Send `window.stack.reorder` and `window.stack.set_active` through the HTTP commands route: they reorder and activate tabs and carry a deprecation. Send `window.move` for a window in a split with no `placement`: it becomes its own tab and the result carries a deprecation with `surface: "window.move.placement=empty"`. `compozy window close --minimize --id <w>` keeps the window open with a warning; `compozy window open --floating --app tasks` opens a tab. Run `compozy cmd-palette invoke window.zoom` and `compozy cmd-palette invoke window.tab.detach`: both return `applied: false` with a deprecation and leave the layout and revision unchanged, and neither ID appears in `compozy cmd-palette list -o json`. `compozy hooks list` on a hook bound to `window_manager.stack.grouped` warns that it never fires.
