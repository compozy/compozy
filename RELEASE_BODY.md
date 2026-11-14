## 0.7.0 - 2026-11-14

### Bug Fixes

- Download the recorded desktop release when the update feed is unreachable (#1643)
- Keep continued child sessions in their parent's session tree (#1647)
- Read the Claude Keychain login past a stale macOS credential file (#1648)
- Promote synthetic wakes with live ACP defaults (#1650)
- Count only closing PR references as issue-triage coverage
- Repair the three web unit tests red on main after tabs and splits (#1672)
- Reopen a window beside its former nested split branch (#1670)
- Bound the composer terminal quote with inner scroll and collapse (#1684)
- Acknowledge layout upgrade notices atomically and toast them once (#1671)
- Make worktree removal responsive and recoverable (#1668)
- Keep pane scroll positions across more tab and split moves (#1674)
- Keep unsaved agent edits when a window command takes the editor away (#1676)
- Resolve native tool supervision follow-ups (#1679)
- Coalesce seam key repeat, admit tab-wide balance by fit, keep drags through own commands (#1675)
- Settle the window-manager follow-ups from the tabs-and-splits delivery (#1677)
- Keep one history entry per open-or-focus and carry the keyboard with own focus changes (#1678)

### Features

- Add a Move under parent picker for session organization (#1646)
- Group CompozyOS menus into submenus with List and Kanban parity (#1652)
- Replace floating windows with tabs and splits in the CompozyOS window manager (#1640)
- **BREAKING:** replace floating windows with tabs and splits in the CompozyOS window manager (#1640)
- Swap split windows from the Window menu, the palette, and shortcuts (#1673)
- Preview Markdown, tables, HTML, notebooks, PDF and media in the CompozyOS Files viewer (#1694)

### Performance Improvements

- Reuse the session writer's transcript encode buffer across appends (#1634)
- Read only triage rows in the periodic triage session sweep (#1636)
- Reload skill watcher config only after its files change (#1627)
- Share hosted MCP projection discovery across session streams (#1629)
- Keep tab clicks outside drag geometry and isolate retained tabs (#1685)

### Testing

- Isolate the telemetry benchmark receiver process (#1639)
- Assert exact agent terminal window identity in E2E and fix helper fallback (#1669)
- Synchronize flaky runtime and browser assertions (#1662)
- Drive the webhook Copy submenu from the keyboard (#1683)
- Assert the unsaved guard dialog contract after Keep editing cancels a Discard (#1691)

### Release Notes

#### Breaking Changes

##### Windows are now tabs and splits

The CompozyOS window manager is now as simple as a browser. Each desktop has one tab bar, a tab holds one window or several windows split side by side, and a window only opens or closes. Floating windows, minimize, zoom, snapping and tabs inside a pane are gone, and with them the windows that could get hidden, overlap or end up off-screen.

- **Tab bar.** Open, close, reorder, pin and reopen tabs the way you do in a browser. Tabs shrink to fit, then scroll with a list of the ones that do not fit. The bar is one keyboard stop: Left, Right, Home and End move between tabs. Right-click a tab for **Pin tab**, **Move left** / **Move right**, **Split with ▸**, **Arrange ▸**, **Move to desktop ▸** and the close set (**Close tab**, **Close other tabs**, **Close tabs to the right**, **Close tabs to the left**, **Close all tabs**), which skips pinned tabs.
- **Splits by drag.** Drag a tab down onto a window and drop it on the side you want. Near the outer edge of the work area the drop covers the whole edge. Drag a window by its header to move it inside the tab, or hold Shift to swap two windows.
- **Resize at the seams only.** Drag the line between two windows, or focus it and use the arrow keys; double-click it to make both sides equal. **Arrange ▸** offers **Main and stack**, **Columns**, **Grid** and **Balance sizes**.
- **Move to new tab replaces maximize.** A window in a split has **Move to new tab** in its header and in the Window menu, or you can drag its header up onto the tab bar.
- **Desktops.** The desktops overview lists each desktop's tabs, reorders desktops by drag, takes tabs dropped on a card and closes a desktop in place after you choose where its tabs go. The desktop dots in the top bar have a menu (**Rename**, **Move left**, **Move right**, **Close desktop**, **New desktop**) and accept a dragged tab. Moving a tab to another desktop leaves you on the desktop you are on. A desktop that holds only New tab placeholders counts as empty, so `compozy desktop delete` removes it without `--destination`.
- **Each screen keeps its own active tab.** Two browsers or app windows on one workspace each have their own active tab, so switching tabs in one never moves the other.
- **Settings › Layouts** edits a saved layout as desktops, their tabs and the splits inside each tab, and imports layouts saved by earlier versions.
- **Agents and scripts** split, unsplit and move with `compozy window move` and `compozy__window_move` (new `--placement tab` and `--tab` / `target_tab_id`), reorder tabs with the new `compozy window reorder`, close with the new scopes `left` and `all`, balance a whole tab with `layout balance --tab`, and get the hook events `window_manager.tab.split`, `window_manager.tab.unsplit` and `window_manager.tab.activated`.

## Migration notes

**Your layout is converted once, on the first start of v0.7.0.** No window, route, navigation history or desktop is lost:

- Tiled windows keep their splits and become tabs. Windows that shared one pane as tabs become adjacent tabs, with the active one in place.
- Floating windows, floating tab groups and minimized windows become tabs after the tiled ones.
- Windows snapped side by side become one split tab when they line up; otherwise each becomes its own tab.
- A window that was zoomed onto its own desktop returns, as its own tab, to the desktop it came from.

**What the upgrade discards:** the positions and sizes of floating windows, island frames, the minimized and zoomed state, the saved spot a minimized or zoomed window would have returned to, the grouping of several tabs inside one pane, the pin of a window that ends up inside a split (a pinned window that becomes its own tab stays pinned), the desktops that zoom created, and undo/redo history.

The notification bell tells you what happened with **Your windows are now tabs** and the number of floating and minimized windows that became tabs, and a toast shows it once. Marking it read once clears it in every window. Agents and scripts read the same status with `compozy layout migration status` (or `migration` in `compozy layout get` and `compozy__layout_get`) and dismiss it with `compozy layout migration ack`.

**If the conversion fails, nothing is lost.** CompozyOS keeps your previous layout exactly as it was stored, opens the workspace on a desktop with one New tab window so you can keep working, and shows **We could not move your layout to tabs** with **Retry** and **Dismiss** (dismissing keeps the saved copy). `compozy layout migration retry` runs the conversion again; it is refused when you changed the layout after the failure, and `--replace` replaces your current layout with the converted one. In the app the notice then offers **Replace current layout**, which asks you to confirm and does the same. The saved copy is kept until a retry succeeds. After a successful conversion the saved copy is removed when you first start v0.8.0. Earlier saved copies set aside by a later upgrade (see below) follow the conversion they belonged to: a copy from a successful conversion is also removed when you first start v0.8.0, and a copy from a failed conversion is kept until you delete its workspace or profile.

**Saved layout profiles and extension layouts** are converted too. Saved profiles are rewritten to layout document version 6 at the first start. Layout documents of version 4 and 5 keep loading everywhere (`compozy layout apply`, `layout validate`, `layout-profile put`, extension `window_layouts`) for good; `compozy layout export` always writes version 6.

**Do not downgrade to v0.6 after upgrading.** v0.6 cannot read a converted layout: it deletes it and opens the workspace on a default desktop, and it rejects saved profiles of version 6 as an unsupported version. The original v0.6 layout is still stored, but v0.6 does not restore it, and upgrading again converts whatever layout you used on v0.6 instead. Nothing stored is lost on the way back up, even when the first conversion failed: if the layout you used on v0.6 differs from the saved copy, the earlier copy is set aside, unchanged, in the same write that saves the new one, and the migration status counts the copies kept in `superseded_backups`. **Retry**, **Dismiss** and `compozy layout migration retry [--replace]` then act on the newest saved copy. If keeping the extra copy would exceed the workspace's storage quota, nothing is changed: every stored layout and copy stays as it was, that workspace's layout answers `window_manager_unavailable` with the diagnostic `snapshot_key_quota`, and CompozyOS keeps serving the other workspaces. If you must go back, export your layouts first with `compozy layout export`.

A layout saved by a newer CompozyOS (layout version 7 or later) is left untouched and is not converted: that workspace's layout answers `window_manager_unavailable` until you run the newer version again. CompozyOS still starts, logs the skipped workspace (`window_manager.partition_reconciliation_skipped`), and serves every other workspace. The same holds for a layout whose upgrade was refused by the storage quota: it is retried on every load, changes nothing while the quota is exceeded, and answers `window_manager_unavailable` with the diagnostic `snapshot_key_quota`. Two cleanups that touch every stored layout clean every layout CompozyOS can use but cannot finish while a layout it cannot read or cannot write (either case) is stored: deleting a session succeeds and removes its windows from every usable layout, and the rest is retried in the background (logged as `session: retained deletion tombstone for window reconciliation retry`); removing a workspace deletes that workspace's own layouts first, whatever their state, then cleans every other usable layout and fails with an internal error (HTTP 500) naming the unusable layout of another workspace, and completes when you repeat it once that layout is usable again.

**Retired surfaces keep working until v0.8.0** and say what to use instead where they are used. Retired commands, command IDs and command fields answer with a `deprecations` entry (`code`, `surface`, `replacement`, `removal_release: "v0.8.0"`, `message`) in the result, which the CLI prints as one `warning:` line on stderr. Retired `config.toml` keys warn once when the configuration loads, retired shortcut bindings are logged as deprecated and marked retired in Settings, and `compozy hooks list` warns about hooks on retired events. Settings API fields and result fields that are now always empty carry no warning in the response; the OpenAPI document marks them deprecated.

| Retired                                                                                                                                                                                                                  | Until v0.8.0                                                                                     | Use instead                                                                                                |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------- |
| `compozy window float`, `compozy__window_float`, command `window.toggle_floating`                                                                                                                                        | Does nothing                                                                                     | `window move --placement tab`                                                                              |
| `compozy window zoom`, `compozy__window_zoom`, `window.zoom`                                                                                                                                                             | Does nothing                                                                                     | `window move --placement tab`                                                                              |
| `compozy window resize`, `compozy__window_resize`, `window.resize`                                                                                                                                                       | Does nothing                                                                                     | `layout resize`                                                                                            |
| `compozy layout frame-resize`, `compozy__layout_frame_resize`, `layout.frame_resize`                                                                                                                                     | Does nothing                                                                                     | `layout resize`                                                                                            |
| `compozy window group`, `compozy__window_group`, `window.stack.group`                                                                                                                                                    | Each window becomes a tab after the target                                                       | `window move --placement tab`                                                                              |
| Command IDs `window.stack.reorder`, `window.stack.set_active`                                                                                                                                                            | Aliases                                                                                          | `window.tab.reorder`, `window.tab.activate`                                                                |
| `window open --floating`, `--rect`, `--restore` (`floating`, `floating_rect`, `restore_window_id`)                                                                                                                       | Opens a tab; ignored; activates that window's tab                                                | `window open`, `window activate`                                                                           |
| `window close --minimize` (`minimize`)                                                                                                                                                                                   | Does nothing: the window stays open                                                              | `window close`                                                                                             |
| `window move --placement floating`, `--rect`                                                                                                                                                                             | Treated as `--placement tab`; ignored                                                            | `window move --placement tab`                                                                              |
| `layout arrange --arrangement stack`, `--frame`, `--group`, `--keep-frames`                                                                                                                                              | Adjacent tabs; ignored                                                                           | `layout arrange`                                                                                           |
| `layout balance --group` (`group_id`)                                                                                                                                                                                    | Alias of `--tab`                                                                                 | `layout balance --tab`                                                                                     |
| `window group --insert-index` (`insert_index`)                                                                                                                                                                           | Ignored, with no deprecation of its own: the command reports only its `window.stack.group` entry | `window move --placement tab` (the entry's replacement), then `window reorder --index` to position the tab |
| `window.move` with an empty or missing `placement` over HTTP/UDS or `compozy__window_move`                                                                                                                               | Treated as `tab`, with the deprecation `window.move.placement=empty`                             | `"placement": "tab"`                                                                                       |
| Palette command IDs `window.minimize`, `window.zoom`, `window.toggle_floating`, `window.merge_all`, `window.tab.detach`, `window.resize`, `layout.frame_resize`, `window.stack.group`                                    | Still invocable as no-ops (`applied: false` with a deprecation); no longer listed                | The live command for the action                                                                            |
| Settings API `GET`/`PATCH /api/settings/window-manager` fields `raise_on_focus`, `drag_away_policy`, `group_move_modifier`, `snap`, `bindings`, and `new_window_policy: "floating"`                                      | Returned with fixed values; ignored on save                                                      | Omit them                                                                                                  |
| `compozy__window_list` `include_minimized`                                                                                                                                                                               | Ignored: every window is listed                                                                  | —                                                                                                          |
| `changes.group_ids`, `stack_grouped`, `stack_ungrouped`; client `stack_active`; `palette_context.window_floating`, `window_stacked`                                                                                      | Always empty or `false`; `window_stacked` mirrors `window_in_split`                              | `tab_ids`, `tab_split`, `tab_unsplit`, `active_tab`, `window_in_split`                                     |
| Hook `window_manager.stack.activated`                                                                                                                                                                                    | Fires together with `tab.activated`                                                              | `window_manager.tab.activated`                                                                             |
| `config.toml` `[window_manager]` `raise_on_focus`, `drag_away_policy`, `group_move_modifier`, `snap.*`, `bindings.top_center`, `bindings.bottom_center`                                                                  | Ignored with one warning                                                                         | Remove the key                                                                                             |
| `new_window_policy = "floating"`                                                                                                                                                                                         | Loads as `"tab"` with a warning                                                                  | `"tab"` or `"beside_focus"`                                                                                |
| Shortcut IDs `window.minimize`, `window.zoom`, `window.toggle_floating`, `window.tile.top-left`, `window.tile.top-right`, `window.tile.bottom-left`, `window.tile.bottom-right`, `window.merge_all`, `window.tab.detach` | Load and do nothing; Settings marks them retired                                                 | Remove the binding; `window.unsplit` replaces `window.tab.detach`                                          |

`window.tile.left`, `right`, `top` and `bottom` now move the focused window to that edge of its tab. The new shortcuts `window.unsplit` (**Move to new tab**), `window.tab.move_left` and `window.tab.move_right` are unbound by default. `compozy window move` now defaults to `--placement tab` instead of `floating`. `compozy window swap` and `compozy__window_swap` swap two windows of the same tab; other pairs are rejected with `window_manager_invalid_command`.

**Accepted breaks, with no compatibility window:**

- Results that contain the layout have a new shape with no version 5 form, because version 5 cannot describe a tab bar of splits: the snapshot (`version: 6`, `desktops[].tabs[]` each with a split tree, `windows[].tab_id`), desktops, windows, layout documents and closed entries, wherever they are returned — HTTP/UDS snapshots and command results, the window-manager stream, `compozy__layout_get`, `compozy__desktop_list`, `compozy__window_list`, CLI JSON output and the generated SDK types. Client views report `active_tab` instead of `stack_active`. `compozy window list` shows `TAB`, `TAB INDEX`, `ACTIVE`, `PINNED` and `MEMBERS`, and its JSON rows carry `tab_id` and `tab_index` instead of `stack_id` and `member_order`.
- The hook events `window_manager.stack.grouped` and `window_manager.stack.ungrouped` have no equivalent. Hooks that subscribe to them still load until v0.8.0, never fire, and `compozy hooks list` warns about them. Use `window_manager.tab.split` and `window_manager.tab.unsplit`.

#### Features

##### Choose a parent for a session from its menu or the command palette

Nesting a session under another used to need a drag, or a submenu that listed only the sessions already loaded in the sidebar.

- **Move under….** A session's row menu and right-click menu open a parent picker that searches the project's sessions by title or agent. Arrow keys and Enter choose the new parent, and the confirmation offers Undo.
- **From the command palette.** With a session window focused, type `move` or `parent` and choose "Move under another session…" to open the same picker.
- **Back to the top level.** On a nested session, the picker offers Make top-level, as the row menu does.
- **Continued and forked sessions stay put.** Their picker explains that they stay linked to the session they came from instead of listing parents. The daemon's refusal now says the same.

##### Swap two windows of a split from the Window menu, the palette, or a shortcut

Two windows of a split could trade places only by dragging one header onto the other with Shift held.

- **Window › Swap.** With a window in a split focused, **Swap ▸** lists **Left**, **Right**, **Up** and **Down** for each side that has a neighbour. The focused window trades places with that neighbour and keeps focus.
- **From the command palette.** Type `swap` and choose **Swap left**, **Swap right**, **Swap up** or **Swap down**. A side with no window does nothing; swaps never wrap around.
- **Your own shortcut.** The four commands (`window.swap.left`, `.right`, `.up`, `.down`) ship unbound. Assign a chord in Settings › Shortcuts. Existing shortcut settings load unchanged.
- **One undo step.** Layout undo puts both windows back.

#### Fixes

##### Claude usage reads the macOS Keychain login past a stale credential file

On macOS, an expired, malformed or unscoped `.credentials.json` left in Claude Code's
config directory no longer reports Claude usage as signed out while Claude Code is
logged in through the Keychain. With Keychain access allowed, CompozyOS reads the
Keychain login through its existing silent, cached path; without consent it asks for
Keychain access instead of asking you to log in again. A usable credential file still
takes precedence and never triggers a Keychain read. Linux and Windows are unchanged.

Migration notes: no action needed. Existing consent settings and credential stores
are preserved.

##### Large terminal quotes in the composer stay a bounded size

Sending a large terminal selection to a conversation no longer fills the screen. The quote in the composer has a bounded height with its own scroll, keeps the terminal's line breaks and spacing, and can be collapsed with its chevron; quotes longer than 12 lines start collapsed. The composer input and controls stay visible on desktop and phone-width windows.

Migration notes: no action needed.

##### Continued child sessions stay under their parent in the session list

Continuing a child session no longer moves the new session to the top of the session list. When an orchestrator continues a worker, for example after a daemon restart stopped it, the continuation now appears in the orchestrator's thread next to the session it continues. This also holds when a session is continued more than once. Continuations of top-level sessions and forks still show as top-level sessions, and the continued session still names its source as "Continued from".

Only placement in the list changes. Permissions, lifecycle and child counts work as before: a continuation is not counted as one of the parent's children and does not stop when the parent stops. Session list and detail reads now include `tree_parent_session_id`, the parent a continuation is listed under. Sessions continued before this release move under their parent the next time the list loads; no migration is needed.

##### Desktop app updates download on networks that block the update feed

When the desktop app cannot reach its update feed, it now downloads the release the daemon already selected straight from the GitHub release and verifies it against the release's signed checksum. A failed update now says which network error stopped the download, such as `net::ERR_CONNECTION_CLOSED`, instead of "The app update could not be installed."

Migration notes: the fix ships in the updater itself. If updating from 0.6.0, 0.6.1, or 0.6.2 keeps failing with a connection error, download this release once from the GitHub release page and install it manually; later updates use the fallback automatically.

##### Hosted MCP sessions share tool discovery

CompozyOS reuses MCP tool discovery across open sessions with the same provider listing scope during each refresh window, reducing repeated discovery and descriptor processing. Sessions retain their own tool permissions and availability checks, and separate workspaces and profiles keep separate listings. Catalog changes retain the existing refresh bound.

##### The layout upgrade notice reads atomically and toasts once

Marking the "Your windows are now tabs" bell notice read now records every listed
upgrade together before updating the stored layouts, so a failure no longer leaves
the bell partly read; if updating a stored layout fails, the next bell read
finishes it. Upgrades over several projects
show one toast naming them instead of one identical toast each, and an upgrade that
was already announced does not toast again after a reload.

Migration notes: no action needed.

##### Native tool supervision follow-ups

Native tools and subagent supervision handle several edge cases more reliably. One failed native-result row no longer stops other replay work. A failed MCP discovery attempt is retried by peers instead of being shared as a complete tool list, while healthy tools stay available. Monitors on child sessions no longer depend on whether you viewed the child. Scheduler and harness heartbeat wakes use the agent's current heartbeat policy. A native approval that expires is reported as `approval_timed_out` with guidance to ask again, instead of looking like an explicit rejection.

Migration notes: no action needed.

##### Panes keep their scroll position across more tab and split moves

Moving a window between tabs and splits keeps your place more reliably. A session
transcript returns to the same text after a tab switch or a width change and no longer
drifts forward once shown; a turn that started while its tab was hidden is shown when you
come back. Lists such as the Files tree keep their offset after a tab drag into a split,
a list beside a detail keeps its offset after a later move, and a list that an in-window
navigation sends to the top right after a tab switch stays at the top. Moving a shown
window into a new tab with `compozy window move --placement tab` keeps its pane open
instead of reloading it.

Migration notes: no action needed.

##### Reopening a window closed beside a nested split restores the whole branch

Reopen closed tab (⌘⇧T) now brings back a window that was closed beside a nested split in its
original slot and share: with A on the left and B over C on the right, closing and reopening A
returns it as the left column beside the B/C branch instead of squeezing it next to B. When the
branch's windows no longer sit together, the window returns beside its former neighbor as before.

Migration notes: no action needed. Windows closed before this release reopen with the previous
single-neighbor behavior.

##### Responsive worktree removal with batch previews

Registered worktrees now leave the active checkout path before a paced background worker deletes their files. Large dependency trees no longer hold the repository lock throughout file deletion. Cleanup pauses during CPU pressure, resumes leftovers on startup and falls back to synchronous removal when staging cannot rename the checkout.

The worktree removal CLI, native tool and HTTP/UDS API accept batches with per-target outcomes and a `dry_run` risk preview. Existing single-target request and response shapes are preserved. Failed fenced reads restore the original record, and repeating removal reconciles abandoned `removing` records without restarting the daemon.

Migration notes: no action needed. Background cleanup may retain staged bytes under the managed worktrees root until CPU pressure subsides.

Collection rediscovers profile-specific roots after restart and only empties CompozyOS-owned trash. Existing trash folders stay untouched, submodule checkouts retain Git's synchronous safety checks, and CLI batch timeouts scale with the number of targets.

Interrupted staging now restores verified checkout bytes before retrying safety checks. Failed restores keep the removal fence and preserve staged changes across retries and daemon recovery.

After staged safety checks succeed, CompozyOS durably records removal approval before clearing Git registration. Recovery uses that approval to finish an interrupted state write without restoring a broken checkout. External Git pruning alone never authorizes deletion of refused staged changes.

Configured worktrees roots reached through directory symlinks support staged removal, recovery and clean rollback without stranding the checkout. Literal percent signs and Unicode directory names remain unchanged when resolving those roots.

##### Seam resizing stops with the key and Balance sizes never collapses a split

Holding an arrow key on a split seam against a slow daemon no longer keeps resizing after the key is released: the steps that wait behind a resize in flight on that seam are sent as one resize. Balance sizes for a whole tab (tab menu, Window › Arrange, command palette) now applies the same fit check as double-clicking a seam: it evens only the splits that can be evened together without showing a split one window at a time. A balance or move made by the same window during a seam drag re-bases the drag instead of cancelling it, the seam's limit state appears with the frame that reaches the clamp, and queued seam input deadlines use a monotonic clock.

Migration notes: no action needed.

##### Idle skill watcher polls use less CPU

CompozyOS reuses workspace and profile skill settings while their configuration files stay unchanged, reducing idle daemon CPU use with many workspaces. Configuration edits, including file creation and deletion, still take effect on the next skill watcher poll.

Migration notes: no action needed.

##### Synthetic wakes reach long turns with provider defaults

CompozyOS promotes queued session replies, monitor wakes, and child completion wakes into long-running turns even when the provider adds ACP defaults to the live runtime. Inputs with an explicit different runtime still wait for their own turn, and debug logs identify runtime-mismatch skips without exposing runtime values.

Migration notes: no action needed.

##### Lower memory allocation while streaming long agent turns

CompozyOS reuses each session writer's transcript encoding buffer, reducing allocation pressure while long, tool-heavy agent turns stream. Transcript contents and write durability stay unchanged, and oversized buffers are released after both successful and failed encodes.

##### Lower background session maintenance overhead

CompozyOS now reads only triage sessions during periodic triage cleanup, reducing background CPU and metadata-cache pressure for large session histories. Deleted session metadata is evicted without waiting for a full session listing, including when a metadata read overlaps deletion. Archive and retention behavior stays unchanged.

##### Agent editors ask before a window command discards unsaved edits

Unsaved edits in the agent editors (Instructions, Heartbeat, Memory, the authored SOUL and HEARTBEAT files, Permissions and the Tools panel) are no longer lost when the window manager takes the editor away. Closing the window or tab with Cmd+W, a close from the menu, or a bulk close now asks first; a bulk close asks once for every affected window. Navigation started from outside the window, such as `compozy window navigate` or a project switch, keeps the draft in memory instead: the editor shows it as unsaved when you return, and saving it goes through the usual conflict check.

Migration notes: no action needed.

##### Back returns to the previous window, and the keyboard follows your focus

Opening or focusing a window from the dock, the palette or a menu now adds exactly one browser history entry, so one Back returns to the window you came from. Before, dock **Files** could overwrite the previous entry and Back stayed on Files.

When you move focus yourself (directional focus, next or previous tab, **Focus last window**, the dock, the palette, a menu, Back or Forward), the keyboard now moves into the window you focused: to the field you last typed in there, or to the window itself. Focus sent from the CLI or another client still moves only the focused window and leaves the keyboard where you are typing.

After a daemon restart, an open browser or desktop client comes back on the desktop it was showing, with the same window focused, if both still exist.

Migration notes: no action needed.

##### Window manager follow-ups for tabs and splits

Several rough edges left after tabs and splits are fixed:

- Editing a shortcut at project scope now saves only what differs from the inherited settings, so later user-level changes still reach that project. Existing project files load the same values; the next project save drops copies equal to the inherited value.
- Agent and extension rows in the command palette update without a reload, and a disabled extension shows "disabled" instead of a health word.
- A terminal window that hit the per-project terminal limit opens a terminal as soon as a slot frees up. Dismissing the limit dialog shows the empty state with its open control.
- Removing a project whose window layout can't be read answers `503 window_manager_unavailable` with the project and ID to retry, instead of a generic 500. Retrying `compozy workspace remove` finishes the removal.
- `compozy hooks list` shows a user-level hook once instead of once per registered project. JSON output is unchanged.
- The tab context menu groups its commands the same way as the other CompozyOS menus.

Migration notes: no action needed.
