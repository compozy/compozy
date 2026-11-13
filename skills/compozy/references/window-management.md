# Window Management

## Contents

- Operating model
- Tabs and splits
- CLI
- Native tools
- HTTP, UDS, and stream
- Layout upgrade
- Raw layout recovery
- Configuration and hooks
- Retired surfaces
- Reserved partitions

CompozyOS manages persistent virtual desktops, their tab bars, the split trees inside tabs, and layout
history as one daemon-authoritative workspace topology. Never edit internal client-state keys or
invent split weights. Use semantic desktop, window, and layout operations; the daemon validates,
normalizes, and commits the complete topology atomically.

## Operating Model

- Each desktop has one ordered tab bar. A tab holds one window or a split tree whose every leaf is a
  window. A window belongs to exactly one tab, and a tab to exactly one desktop. A desktop always
  has at least one tab; a tab always has at least one window.
- A window is open or closed. It has no position, size, z-order, minimized, zoomed, or floating
  state; every window is visible while its tab is active. "Move to new tab" (placement `tab`)
  takes a window out of a split; it replaces maximize and zoom.
- Split nodes have an `axis` (`horizontal` places children side by side, `vertical` stacks them)
  and `weights` that are finite, positive, and sum to 1. A split left with one child is replaced by
  that child. Same-axis nested splits are flattened. Resizing happens only at a seam between split
  members (`layout resize`).
- When a split cannot fit its members' pixel minimums, clients show one member at a time with a
  member switcher. That is presentation only; the durable topology does not change.
- Every durable mutation is compare-and-swap guarded by the workspace `revision`. A successful
  mutation advances it once and records one event. Layout/topology changes add one history entry;
  route navigation and tab activation do not enter or clear layout history.
- A stale `expected_revision` is still applied when the command carries a `rebase` guard whose
  named identities (`window_id`, `source_node_id`, `target_node_id`, `tab_id`, `split_id`,
  `boundary_index`) still hold; the result then reports `rebased_from`. A `tab_id` guard fails
  with `rebase.tab_missing` when the tab is gone. Without a passing guard the command is a
  `window_manager_revision_conflict`.
- `desktop.switch` and `window.focus` require an explicit connected `client_id`. They never select an
  implicit foreground browser.
- The active tab is per client: `active_tab` (desktop ID → tab ID) in each client view. A client's
  value wins while that tab exists, then the desktop's durable `active_tab_id`, then the first tab.
  `active_tab_id` is only the default for new clients and for opens with no client. Clients never
  follow each other's active tab.
- Preview, validation, rejected commands, and no-ops do not persist or emit topology events.
- Snapshots and layout documents are version 6. The wire snapshot carries no `history` body; undo and
  redo stay daemon-internal, and the snapshot reports `closed_entry_count` instead of reopen bodies.
  Stored version 3, 4, and 5 snapshots are converted to version 6 on load (see Layout upgrade).
- On load the daemon also drops windows whose app is no longer registered, renormalizes the topology,
  rewrites a retired Settings sub-route to `/settings`, and persists the result once under the next
  revision. After a successful persist it logs `windowmanager.snapshot_reconciled` with `workspace_id`,
  `profile_id`, the sorted dropped app IDs (`dropped_apps`), the rewritten-route count
  (`rewritten_routes`), and the new `revision`, counting the current arrangement, its history, and its
  closed windows. Persisted `window_layout` resources follow the same registered-app rule.

Snapshot shape (abridged):

```json
{
  "version": 6,
  "workspace_id": "ws_1",
  "revision": 44,
  "desktops": [
    {
      "id": "dsk_1",
      "name": "Desktop 1",
      "order": 0,
      "active_tab_id": "tab_k3",
      "tabs": [
        {
          "id": "tab_k3",
          "pinned": false,
          "root": {
            "id": "n_7",
            "kind": "split",
            "axis": "horizontal",
            "weights": [0.65, 0.35],
            "children": [
              { "id": "n_1", "kind": "leaf", "window_id": "win_a1" },
              { "id": "n_2", "kind": "leaf", "window_id": "win_b2" }
            ]
          }
        }
      ]
    }
  ],
  "windows": {
    "win_a1": {
      "id": "win_a1",
      "app": "session",
      "desktop_id": "dsk_1",
      "tab_id": "tab_k3",
      "route": { "pathname": "/sessions/s1", "search": {} },
      "nav_stack": []
    }
  },
  "closed_entry_count": 0,
  "overrides": {},
  "updated_at": "2026-11-12T15:00:00Z"
}
```

Tab, node, and window IDs are opaque. Never parse them; read them from `changes` or the snapshot.

Read the current revision before mutating:

```bash
compozy layout get --workspace <workspace-id> -o json
```

Pass that value as `--revision` to each CLI mutation. On
`window_manager_revision_conflict`, read again and recompute the intended semantic command. Do not
blindly replay stale node IDs or weights.

## Tabs And Splits

- Pinned tabs form a contiguous prefix of the bar. Reorder clamps an index into the tab's own
  pinned or unpinned range. `pin`/`unpin` act on the window's whole tab.
- A pinned tab dropped into another tab takes the target tab's pin state. A window moved out of a
  pinned tab becomes an unpinned tab.
- `nav_stack` holds a window's route ancestors, oldest first; `route` stays the current leaf.
- A tab's lead window (its title, and focus on activation) is the member most recent in that
  client's focus order, else the first leaf. There is no durable per-tab focus.

Closing and reopening:

- Every close pushes exactly one closed entry, whatever the scope: one command, one revision, one
  entry. Closing a split member gives its space to its sibling; closing the last window of a tab
  closes the tab; closing the last tab of a desktop leaves one New Tab window.
- `close --scope tab` (default) closes one window. It is refused with
  `window_manager_window_pinned` when that window is the last one of a pinned tab (unpin first);
  a member of a pinned split tab with other members closes normally. `group` closes every window of
  the window's tab, pinned or not. `others`, `right`, `left`, and `all` close unpinned tabs of the
  desktop: every other one, those to the right, those to the left, every one.
- `window reopen` pops the newest entry and recreates its windows with their original IDs, routes,
  and nav stacks. A closed tab returns to its former bar position with its pin. A closed split member
  returns into its former split when that split still exists, else re-splits beside the neighbor it
  had on the same side, else comes back as a tab right after its former tab or at the end of the bar.
  Members still live are skipped. An empty reopen history succeeds without applying.
- Window IDs are reserved across live windows **and** windows retained in closed entries. Reusing a
  reserved ID fails with `window_manager_invalid_command`; omit the ID and the daemon generates a
  free opaque one, readable from the command result's changed window IDs.

## CLI

Desktop lifecycle:

```bash
compozy desktop list --workspace <workspace-id> -o json
compozy desktop create --workspace <workspace-id> --revision <revision> --name Build -o json
compozy desktop update --workspace <workspace-id> --revision <revision> --id <desktop-id> --name Review
compozy desktop reorder --workspace <workspace-id> --revision <revision> --id <desktop-id> --order 0
compozy desktop delete --workspace <workspace-id> --revision <revision> --id <desktop-id> \
  --destination <desktop-id>
```

Deleting the final desktop is rejected. Deleting a desktop with tabs requires a distinct explicit
destination; its tabs move there in the same transaction. A desktop whose only tabs are New tab
placeholders (app `new-tab`) counts as empty: omit `--destination`, and the placeholders are dropped,
not moved.

Client-local presentation:

```bash
compozy desktop clients register --workspace <workspace-id> --client <stable-client-id> -o json
compozy desktop switch --workspace <workspace-id> --revision <revision> \
  --client <stable-client-id> --id <desktop-id>
compozy window focus --workspace <workspace-id> --revision <revision> \
  --client <stable-client-id> --direction right
```

Focusing a window in a non-active tab makes that tab the client's active tab. Focus-driven
activation reaches the durable `active_tab_id` through a 2-second coalescer (command
`window.tab.activate`, actor `system/active_coalescer`), outside undo history.

Use `compozy desktop clients list|register|unregister` to manage live presentation identities. Browser
clients persist only their random stable client ID locally; topology and revisions stay in the daemon.
Each client view advances its own monotonic `presentation_revision`, independently of the workspace
topology revision. Re-registering an existing stable ID without an active desktop preserves its
repaired view and is a no-op when nothing changed.

Windows and tabs:

```bash
compozy window list --workspace <workspace-id> -o json
compozy window open --workspace <workspace-id> --revision <revision> --app tasks \
  --pathname /tasks --search-json '{}'
compozy window open --workspace <workspace-id> --revision <revision> --app tasks \
  --pathname /tasks --search-json '{}' --stack-target <window-id>
compozy window navigate --workspace <workspace-id> --revision <revision> --id <window-id> \
  --mode push --pathname /tasks/<task-id> --search-json '{"tab":"runs"}'
compozy window navigate --workspace <workspace-id> --revision <revision> --id <window-id> --mode pop
compozy window activate <window-id> --workspace <workspace-id> --revision <revision> \
  --client <stable-client-id>
compozy window reorder <window-id> --workspace <workspace-id> --revision <revision> --index 0
compozy window pin <window-id> --workspace <workspace-id> --revision <revision>
compozy window unpin <window-id> --workspace <workspace-id> --revision <revision>
compozy window close --workspace <workspace-id> --revision <revision> --id <window-id> --scope others
compozy window reopen --workspace <workspace-id> --revision <revision>
```

- Where a new window lands follows `new_window_policy`: `tab` (default) adds a tab right after the
  opener's active tab and activates it for the opener; `beside_focus` splits it to the right of the
  client's focused window. An open with no client places the tab right after the tab of the window
  the daemon names as its anchor, else after the desktop's default active tab. Explicit placement
  wins: `--stack-target <window-id>` opens a new tab right after that window's tab, and `--tiled`
  opens a split member to the right of the focused window.
- `activate` makes the window's tab the desktop's durable default and, with `--client`, that
  client's active tab. It is the CLI form of `window.tab.activate`.
- `reorder` moves the window's tab to a zero-based `--index`, clamped to the pinned or unpinned range.
- `navigate --mode replace|push|pop` (default `replace`). `push` records the current route in
  `nav_stack` before moving; `pop` returns to the newest `nav_stack` entry and rejects `--pathname`
  and `--search-json`. Popping an empty stack succeeds without applying.
- `close --scope tab|group|others|right|left|all` (default `tab`); see Closing and reopening.
- `window list` reports, in every output mode, `ID, APP, DESKTOP, TAB, TAB INDEX, ACTIVE, PINNED,
  MEMBERS, PATHNAME, NAV DEPTH`; JSON rows carry `tab_id`, `tab_index`, `active`, `pinned`, `members`.

Open jobs and triggers with `--app automations` (`--pathname /automations/jobs/<job-id>` or
`/automations/triggers/<trigger-id>`). Never emit the retired app ids `jobs` or `triggers` in commands
or `window_layout` resources; they are still translated to `automations` with a warning. After that
translation, `window open` (and its preview) with an app id that is not registered is refused with
`window_manager_invalid_command` ("unregistered window app") on every surface.

Splitting, unsplitting, and moving:

```bash
# Split: put window B to the right of window A, inside A's tab.
compozy window move --workspace <workspace-id> --revision <revision> --id <window-b> \
  --desktop <desktop-id> --target <window-a> --placement right
# Full-height member at the left edge of a tab.
compozy window move --workspace <workspace-id> --revision <revision> --id <window-b> \
  --desktop <desktop-id> --tab <tab-id> --placement left
# Unsplit: window B becomes its own tab right after the tab it came from.
compozy window move --workspace <workspace-id> --revision <revision> --id <window-b> \
  --desktop <desktop-id> --placement tab
# Move the whole tab (split included) to another desktop.
compozy window move --workspace <workspace-id> --revision <revision> --id <window-id> \
  --desktop <other-desktop-id> --group
compozy window swap --workspace <workspace-id> --revision <revision> \
  --first <window-id> --second <window-id>
```

- Edge placements are `left`, `right`, `top`, `bottom` (`before` ≡ `left`, `after` ≡ `right`). With
  `--target` the window becomes a split member at that edge of the target window, in the target's
  tab. With `--tab` and no `--target` it becomes a member at that root edge of the tab.
- `--placement tab` (the default) makes the window its own tab on `--desktop`: right after
  `--target`'s tab when given, else right after its source tab on the same desktop, else at the end
  of the bar. `center` with `--target` is the same as `tab` with that target.
- `--group` moves the whole tab instead of the one window: to the destination desktop, or merged at
  the target edge. A single-window tab always moves whole. Moving a tab to another desktop does not
  switch any client's active desktop.
- Moving a window onto itself, or an edge placement with neither `--target` nor `--tab`, is
  `window_manager_invalid_command`.
- `swap` exchanges two windows of the same tab; windows in different tabs are rejected with
  `window_manager_invalid_command`.
- With a bound client, `window navigate --client`, `window focus --id`, and `window activate
  --client` also activate the window's desktop for that client.

Layout operations:

```bash
compozy layout arrange --workspace <workspace-id> --revision <revision> --desktop <desktop-id> \
  --window <window-a> --window <window-b> --arrangement horizontal
compozy layout arrange --workspace <workspace-id> --revision <revision> --resource <resource-id>
compozy layout resize --workspace <workspace-id> --revision <revision> \
  --split <node-id> --boundary 0 --delta 0.05
compozy layout balance --workspace <workspace-id> --revision <revision> --tab <tab-id>
compozy layout balance --workspace <workspace-id> --revision <revision> --split <node-id>
compozy layout undo --workspace <workspace-id> --revision <revision>
compozy layout redo --workspace <workspace-id> --revision <revision>
compozy layout watch --workspace <workspace-id> -o jsonl
compozy layout watch --workspace <workspace-id> --client <stable-client-id> -o jsonl
```

- `--arrangement` accepts `horizontal`, `vertical`, `grid`, or `main_stack`. It builds one split tab
  from the listed windows, placed where the first listed window's tab was; source tabs left empty
  are removed. `main_stack` gives the first `--window` the main column and splits the rest
  vertically beside it.
- `layout resize` moves one boundary of one split node in weight space; `--delta` is the weight
  transferred from the right child to the left one, with a 0.01 floor per child.
- `layout balance --tab` equalizes every split in the tab; `--split` equalizes one split.

`--resource` is exclusive with inline arrangement fields. Declarative `window_layout` resources are
data-only topology templates: strict, versioned, workspace-bound when scoped locally, with `any`,
`landscape`, or `portrait` aspect variants plus participant slots and an overflow policy (both stored
and validated, not applied). A resource cannot execute code or receive pointer events. Discover them
through `compozy__resources_list`; they may be global or workspace-scoped, and a workspace resource
wins when IDs collide. Applying one is a full `layout.replace` of the workspace through the same
preview, revision/CAS, validation, commit, event, and history pipeline.

## Native Tools

The lazy `compozy__window_manager` toolset provides desktop, window, and layout operations with the same
workspace, revision, client, diagnostic, and risk contracts as the CLI/API. Resolve
`compozy__tool_info` for an exact tool before calling it. Use `window_manager.read` for reads/previews and
`window_manager.write` for mutations. Tool inputs are strict: an unknown field is rejected.

| Operation               | Tool                                                                                                                                                     | Inputs that matter                                                                                                                                                             |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Inspect                 | `compozy__layout_get`, `compozy__window_list`, `compozy__desktop_list`, `compozy__desktop_clients`                                                       | `layout_get` includes `migration` while the upgrade notice is unacknowledged; `window_list` rows carry `tab_id`                                                                |
| Open                    | `compozy__window_open`                                                                                                                                   | `app`, `route`, `desktop_id`, `stack_target_window_id` (tab after that window's tab), `insert_tiled` (split right of focus)                                                    |
| Close                   | `compozy__window_close`                                                                                                                                  | `window_id`, `scope` `tab`/`group`/`others`/`right`/`left`/`all`                                                                                                               |
| Activate, reorder, pin  | `compozy__window_activate`, `compozy__window_reorder` (`index`), `compozy__window_pin` (`pinned: true                                                    | false`)                                                                                                                                                                        | act on the window's tab |
| Split, unsplit, move    | `compozy__window_move`                                                                                                                                   | `window_id`, `destination_desktop_id`, `placement` (`tab`, `left`, `right`, `top`, `bottom`, `before`, `after`, `center`), `target_window_id` or `target_tab_id`, `move_group` |
| Swap                    | `compozy__window_swap`                                                                                                                                   | two windows of the same tab                                                                                                                                                    |
| Seams                   | `compozy__layout_resize` (`split_id`, `boundary_index`, `delta`), `compozy__layout_balance` (`tab_id` or `split_id`)                                     |                                                                                                                                                                                |
| Arrange                 | `compozy__layout_arrange`                                                                                                                                | `window_ids`, `arrangement`, or `resource_id`                                                                                                                                  |
| Navigate, reopen, focus | `compozy__window_navigate` (`mode`), `compozy__window_reopen`, `compozy__window_focus` (needs `client_id`)                                               |                                                                                                                                                                                |
| Documents               | `compozy__layout_export`, `compozy__layout_validate`, `compozy__layout_apply`, `compozy__layout_preview`, `compozy__layout_undo`, `compozy__layout_redo` | documents version 4, 5, or 6 accepted; export writes 6                                                                                                                         |

A tool result is `{workspace_id, command_id, revision, applied, changes, diagnostics, client?,
rebased_from?}` plus `deprecations` when a retired surface was used. `changes` carries
`desktop_ids`, `window_ids`, `tab_ids`, `node_ids`, `client_ids`, `tab_split` (tabs that went from
one member to several), and `tab_unsplit` (tabs that went from several members to one, or
dissolved).

## HTTP, UDS, And Stream

HTTP and UDS expose identical workspace routes under:

```text
/api/workspaces/{workspace_id}/window-manager
```

The surface includes snapshot, preview, commands, client list/register/unregister, layout
export/validate/apply, layout profiles, migration status, and the WebSocket stream. A command body is
`{workspace_id, command_id, expected_revision, payload, client_id?, rebase?, actor?, origin?}`; the
result is `{snapshot, applied, changes, diagnostics?, client?, rebased_from?, deprecations}` with the
revision at `snapshot.revision`. Payload decoding is strict: unknown fields are HTTP 422
`window_manager_invalid_command`, `mode: "pop"` rejects a non-empty `route`.

Command IDs: `desktop.create|update|reorder|switch|delete`, `window.open|navigate|close|focus|move|swap|pin|reopen`,
`window.tab.reorder` (`{window_id, index}`), `window.tab.activate` (`{window_id}`),
`layout.arrange|resize|balance|undo|redo|replace`. Client views carry `active_tab` and
`palette_context.window_in_split`.

Layout profile records use
`GET /layout-profiles` and `PUT|DELETE /layout-profiles/{profile_id}` below the workspace route. A
list contains global records plus records scoped to that workspace; writes cannot address another
workspace. HTTP profile mutations require a loopback listener, while UDS exposes the same request
and response contract.

The CLI covers the same records:

```bash
compozy layout-profile list --workspace <workspace-id> -o json
compozy layout-profile get <profile-id> --workspace <workspace-id> -o json
compozy layout-profile put <profile-id> --workspace <workspace-id> \
  --scope workspace|global --file profile.json --expected-version <n> -o json
compozy layout-profile delete <profile-id> --workspace <workspace-id> --expected-version <n> -o json
```

`--expected-version 0` creates; a non-zero value replaces or deletes that exact version and fails on
a concurrent write. `--scope` defaults to `workspace`. `get` filters the visible list, because there
is no per-profile read route; the list is complete and unpaginated, and it never contains another
workspace's records. Stored profiles saved before v0.7.0 are rewritten to document version 6 at boot.

Pass `--client <stable-client-id>` in the CLI, or an optional registered `client_id` over HTTP/UDS,
to bind the stream to one presentation view. Its initial
snapshot contains that client fence; later client frames carry only that ID and a strictly newer
`presentation_revision`, while topology event frames follow the workspace revision. An unbound
stream receives topology only. Never apply an equal or older client frame. If a subscriber is
evicted as slow, reconnect and replace local state from the next fence. If the daemon lost the
transient client registration, register the same stable ID before reconnecting. Never merge a
reconnect snapshot with an unfenced local mirror. Acknowledging or retrying the layout upgrade
publishes a `snapshot` frame to every subscriber of the workspace.

## Layout Upgrade

The first daemon of v0.7.0 converts each stored version 3, 4, or 5 snapshot to version 6:

- Tiled trees become tabs; a stack keeps its active member in place and its other members become
  adjacent tabs; floating windows, floating stacks, and minimized windows become single-window tabs
  after the tiled tabs; separately snapped tiles merge into one split tab when straight cuts
  separate them; a desktop created by zoom merges back into the desktop the window came from.
- Discarded: floating rectangles, island frames, minimized and zoomed flags, return anchors, the
  grouping of several tabs in one pane, the pin of a window that ends up inside a split tab, and
  undo/redo history. No window, route, navigation history, or desktop is lost.
- The original bytes are kept under a backup key in the same transaction. If conversion fails, the
  workspace opens on a default desktop with one New Tab window and the original stays preserved.

Read and recover through the migration status:

```bash
compozy layout migration status --workspace <workspace-id> -o json
compozy layout migration ack --workspace <workspace-id>
compozy layout migration retry --workspace <workspace-id> [--replace]
```

HTTP/UDS: `GET …/window-manager/migration`, `POST …/window-manager/migration` with
`{"action":"ack"}` or `{"action":"retry","replace":false}`; `replace` is optional and only
meaningful for `retry`.

```json
{
  "state": "migrated",
  "from_version": 5,
  "at": "2026-11-12T15:00:00Z",
  "acknowledged": false,
  "summary": {
    "desktops": 2,
    "tabs": 7,
    "windows": 9,
    "floating_to_tabs": 3,
    "minimized_to_tabs": 1,
    "islands_merged": 1,
    "pins_dropped": 1,
    "zoom_desktops_merged": 0
  }
}
```

- `state` is `none`, `migrated`, or `failed`. A failed status carries `error` (`code`, `message`)
  and `failure_revision`, the layout revision written at the failure. The field is omitted at
  revision 0, which is the revision of the fallback layout a failed upgrade writes, so it is
  normally absent: treat a missing `failure_revision` as 0. `error.code` is the
  converter's own topology diagnostic code when it reports one, otherwise
  `migration.conversion_failed`. `error` is present only when `state` is `failed`; a successful
  retry clears `error` and `failure_revision`.
- `superseded_backups` (integer, omitted when 0) is the number of earlier backups currently kept
  for the profile after later upgrades set them aside (see the downgrade bullet).
- A successful conversion appears in the top bar bell as `kind: "layout-upgrade"`, titled
  "Your windows are now tabs", with the floating/minimized window and removed-pin counts. The bell
  includes every source profile across registered workspaces; `profile` and `receipt_profile`
  select acknowledgement receipts, not the source population. Failed conversions stay inline.
- Reading the bell item or clearing its captured bell snapshot acknowledges that conversion for
  every client and surface, including its source profile. A later successful conversion has a new
  occurrence. `ack` through the migration route or CLI also removes the bell item at the next read
  and dismisses the notice for every client; it deletes nothing.
- `retry` re-runs the conversion from the preserved original. It is valid only for `failed`;
  otherwise `window_manager_invalid_command`. When the current layout changed after the failure,
  it is refused with `window_manager_revision_conflict` unless `--replace` (`"replace": true`) is
  given, which replaces the current layout. A retry that fails again keeps the original, updates
  `error`, and resets `acknowledged` to `false`.
- The same object appears as `migration` on the snapshot and in `compozy__layout_get` until
  acknowledged. Never delete or edit a backup yourself: a failed workspace keeps it until a retry
  succeeds; a migrated workspace's backup is removed at the first boot of v0.8.0. A set-aside
  backup follows the conversion it belonged to: one from a `migrated` conversion is removed at the
  first boot of v0.8.0 by the same rule, one from a `failed` (or unrecorded) conversion is kept
  until its workspace or profile is removed. Identical bytes set aside more than once are one copy;
  its disposition is `failed` if any occurrence came from a failed or unrecorded conversion, and a
  later success does not restore its expiry.
- Downgrading to v0.6 after the upgrade is not supported: v0.6 cannot read a version 6 snapshot and
  replaces it with a default layout. The backup key is untouched by v0.6. The next upgrade
  converts the layout v0.6 wrote, whatever the earlier status was. When that value differs from the
  backup, the earlier backup is set aside unchanged under a content-addressed key in the same write,
  the new value becomes the backup, and `status`, `ack`, and `retry [--replace]` act on the newest
  backup; `superseded_backups` counts the kept copies. No earlier status refuses it and nothing is
  lost. The one refusal is the quota: if the extra keys would exceed the workspace's client-state
  key quota, the whole re-upgrade is refused and nothing is written: the
  workspace answers `window_manager_unavailable` (HTTP 503) with the diagnostic
  `snapshot_key_quota`, every stored value stays intact, and boot continues for other workspaces.
  Report it to the operator.
- A stored snapshot newer than version 6 (written by a later CompozyOS) is never converted or
  rewritten: that workspace answers `window_manager_unavailable` (HTTP 503) while the daemon starts,
  logs `window_manager.partition_reconciliation_skipped`, and serves every other workspace. That
  holds at boot only. A re-upgrade refused by the key quota behaves the same way: it is retried on
  every load, writes nothing while the quota is exceeded, and answers HTTP 503
  `window_manager_unavailable` with `snapshot_key_quota`. For either kind of unusable partition,
  session-deletion and workspace-unregistration cleanup process every usable partition first, then
  report the unavailable ones. A session deletion succeeds and the remaining cleanup is retried in
  the background. Workspace unregistration purges the removed workspace's own partitions first
  (whatever their state), so only an unusable partition of another workspace or profile blocks it:
  it fails with HTTP 500 naming that partition and completes when repeated once it is usable. Report it to the operator; do not try to repair it.

## Raw Layout Recovery

Semantic commands are the normal path. Raw layout replacement is the validated recovery escape
hatch:

```bash
compozy layout export --workspace <workspace-id> -o json > layout.json
compozy layout validate --workspace <workspace-id> --file layout.json -o json
compozy layout apply --workspace <workspace-id> --revision <revision> --file layout.json -o json
```

A document is `{version: 6, workspace_id, desktops, windows, overrides}`: the snapshot without
`revision`, closed entries, history, `updated_at`, and `migration`. Export always writes version 6
and omits history and closed entries, so a raw round trip never restores reopen history. Documents
of version 4 and 5 are accepted everywhere a document is read (`layout apply`, `layout validate`,
`layout-profile put`, extension `window_layouts`, stored profiles) and converted with the upgrade
rules above; this is permanent. Retired `jobs`/`triggers` windows in a version 4 document become
`automations`. Any other version fails validation with `window_manager_invalid_topology` and the
`topology.unsupported_version` diagnostic.

Validate never writes. Apply binds the document to the requested workspace,
validates the complete topology, preserves current routes for surviving window IDs, and performs one
atomic replacement. There is no raw key-value fallback.

## Configuration And Hooks

Global defaults live under `[window_manager]` with nested `gaps`, `shortcuts`, and `global_shortcuts`:
`new_window_policy` (`tab` | `beside_focus`), `small_viewport_policy` (`stack` | `reject`),
`focus_policy`, `focus_wrap`, `focus_follows_pointer`, `swap_modifier`, `history_limit`,
`nav_stack_limit`, `closed_entry_limit`, `desktop_transition`, `gaps.*`.
Validated config updates hot-apply atomically; an invalid update leaves the active defaults unchanged.
Use `GET/PATCH /api/settings/window-manager` for the typed Settings surface. Layout profiles
(any/landscape/portrait) are `window_layout` resources. Workspace layout documents may carry typed
`overrides` without changing other workspaces.

The Settings PATCH preserves its section echo and adds `apply`, the daemon's application receipt.
Inspect `apply.applied`, `next_action`, `warnings`, and `partial_failures`; HTTP success alone does
not prove live application. An unchanged file still retries a pending scoped live application;
a global settings reload is not needed to retry a Layouts PATCH. For a behavior-only `config` save, pass `preserve_shortcuts: true` to
retain the current local and global shortcut maps atomically. Explicit top-level shortcut maps still
replace their respective maps. Omitting the flag retains full-config replacement semantics.
Zero is valid for every gap: `inner` controls spacing between split members; the other four values
are insets.

Shortcut values are one chord string or an array; `""` and `[]` disable an action. Indexed family
keys accept ranges: `desktop.switch = "control+Digit1..9"` and
`window.tab.jump = "control+alt+Digit1..8"`. The daemon expands ranges and validates defaults plus
overrides as one map. `GET /api/settings/window-manager` returns `defaults` and `effective`; use
`compozy config get window_manager -o json` for the same discovery through the CLI. Writes through
`config set` accept a scalar or JSON string array and reject the complete mutation on any collision.
Window shortcuts: `window.tile.left|right|top|bottom` move the focused window to that root edge of
its tab; `window.unsplit` (Move to new tab), `window.tab.move_left`, and `window.tab.move_right` are
unbound by default; `window.tab.new|next|previous|last|reopen|jump.1–8`, `window.focus.*`,
`layout.arrange.*`, `layout.balance`, `layout.undo|redo`, `desktop.*`, and
`window.move_to_desktop.1–9` keep their meaning.

`global_shortcuts` maps one registry command ID to one chord; `palette.summon.global` defaults to
`meta+shift+Space`. Electron shells report per-client registration truth through their window-manager
client context. Pass `client_id` to the Settings read to distinguish intended, active, in-use,
permission-blocked, and unsupported states. Browser clients cannot report registrations.

`nav_stack_limit` (default 50, range 1..200) caps each window's `nav_stack`; `closed_entry_limit`
(default 20, range 1..100) caps retained closed entries. Both are write-time caps resolved from the
effective workspace config when the mutation runs, evicting the oldest value. Lowering a live limit
is not retroactive: it applies to the next relevant mutation, not to stored state.

Committed operations emit async hooks with the payload `{event, timestamp, workspace_id, revision,
command_id, changes, actor, origin}`:

- `window_manager.layout.applied`, `window_manager.desktop.created`, `window_manager.desktop.deleted`,
  `window_manager.window.moved`, `window_manager.window.opened` (`window.open`, `window.reopen`),
  `window_manager.window.closed` (every close scope, carrying all removed window IDs).
- `window_manager.tab.split` when `changes.tab_split` is non-empty, `window_manager.tab.unsplit`
  when `changes.tab_unsplit` is non-empty.
- `window_manager.tab.activated` for explicit `window.tab.activate` and the coalesced focus commit.

Preview, pointer motion, navigation, client focus, per-client tab flips, and desktop switching emit
no extension hooks.

Stable public errors include `window_manager_revision_conflict`,
`window_manager_invalid_command`, `window_manager_invalid_topology`,
`window_manager_destination_required`, `window_manager_history_boundary`,
`window_manager_slow_consumer`, `window_manager_window_not_found`, `window_manager_unavailable`
(a stored snapshot newer than the daemon; it is left untouched), and `window_manager_window_pinned`
(`close --scope tab` on the last window of a pinned tab — unpin it or use `--scope group`). Preserve
structured diagnostics and entity conflicts when reporting failures.

## Retired Surfaces

Floating windows, minimize, zoom, island frames, and per-pane stacks were removed in v0.7.0. Their
surfaces still decode through v0.7.x so older callers do not break, and are deleted in v0.8.0. Never
emit them. When a retired command, command ID, or command field is used, the result's
`deprecations[]` names it:

```json
{
  "code": "window_manager_surface_deprecated",
  "surface": "compozy__window_zoom",
  "replacement": "compozy__window_move with placement \"tab\"",
  "removal_release": "v0.8.0",
  "message": "Zoom was removed. Move the window to its own tab instead."
}
```

A retired field reports only when it carries a non-zero value. CLI verbs print one `warning:` line
per surface to stderr and exit 0. Retired `config.toml` keys warn once when the configuration loads,
retired shortcut bindings are logged as deprecated, and `compozy hooks list` warns about hooks on
retired events. Settings API fields and always-empty result fields carry no warning in the response;
the OpenAPI document marks them deprecated.

| Retired                                                                                                                                                                                                                  | Behavior until v0.8.0                                                                            | Use instead                                                                                                |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------- |
| `compozy__window_float`, `window float`, command `window.toggle_floating`                                                                                                                                                | no-op, `applied: false`                                                                          | `window move --placement tab`                                                                              |
| `compozy__window_zoom`, `window zoom`, `window.zoom`                                                                                                                                                                     | no-op, `applied: false`                                                                          | `window move --placement tab`                                                                              |
| `compozy__window_resize`, `window resize`, `window.resize`                                                                                                                                                               | no-op, `applied: false`                                                                          | `layout resize`                                                                                            |
| `compozy__layout_frame_resize`, `layout frame-resize`, `layout.frame_resize`                                                                                                                                             | no-op, `applied: false`                                                                          | `layout resize`                                                                                            |
| `compozy__window_group`, `window group`, `window.stack.group`                                                                                                                                                            | each listed window becomes its own tab right after the target's tab; `insert_index` ignored      | `window move --placement tab`                                                                              |
| `window.stack.reorder`, `window.stack.set_active`                                                                                                                                                                        | aliases                                                                                          | `window.tab.reorder`, `window.tab.activate`                                                                |
| `window open --floating` (`floating`)                                                                                                                                                                                    | opens a tab                                                                                      | `window open`                                                                                              |
| `window open --rect` (`floating_rect`), `window move --rect`                                                                                                                                                             | ignored                                                                                          | —                                                                                                          |
| `window open --restore` (`restore_window_id`)                                                                                                                                                                            | activates that window's tab; nothing opens                                                       | `window activate`                                                                                          |
| `window close --minimize` (`minimize`)                                                                                                                                                                                   | the window is **not** closed; `applied: false`                                                   | `window close`                                                                                             |
| `window move --placement floating`; an empty or missing `placement` over HTTP/UDS or `compozy__window_move`                                                                                                              | treated as `tab`; the empty one reports `window.move.placement=empty`                            | `--placement tab`                                                                                          |
| `layout arrange --arrangement stack`                                                                                                                                                                                     | listed windows become adjacent tabs                                                              | `window move --placement tab`                                                                              |
| `layout arrange --frame`, `--group`, `--keep-frames`                                                                                                                                                                     | ignored                                                                                          | `layout arrange`                                                                                           |
| `layout balance --group` (`group_id`)                                                                                                                                                                                    | alias of `--tab`                                                                                 | `layout balance --tab`                                                                                     |
| `window group --insert-index` (`insert_index`)                                                                                                                                                                           | ignored, with no deprecation of its own: the command reports only its `window.stack.group` entry | `window move --placement tab` (the entry's replacement), then `window reorder --index` to position the tab |
| Palette command IDs `window.minimize`, `window.zoom`, `window.toggle_floating`, `window.merge_all`, `window.tab.detach`, `window.resize`, `layout.frame_resize`, `window.stack.group`                                    | invocable as no-ops (`applied: false`, `deprecations`); not listed                               | the live command, or none                                                                                  |
| Settings API `GET`/`PATCH /api/settings/window-manager` fields `raise_on_focus`, `drag_away_policy`, `group_move_modifier`, `snap`, `bindings`, `new_window_policy: "floating"`                                          | returned with fixed values; ignored on save                                                      | omit them                                                                                                  |
| `compozy__window_list` `include_minimized`                                                                                                                                                                               | ignored; every window is listed                                                                  | —                                                                                                          |
| `changes.group_ids`, `stack_grouped`, `stack_ungrouped`; client `stack_active`; `palette_context.window_floating`, `window_stacked`                                                                                      | always empty / `{}` / `false`; `window_stacked` mirrors `window_in_split`                        | `tab_ids`, `tab_split`, `tab_unsplit`, `active_tab`, `window_in_split`                                     |
| Hook `window_manager.stack.activated`                                                                                                                                                                                    | fires together with `tab.activated`                                                              | `window_manager.tab.activated`                                                                             |
| Hooks `window_manager.stack.grouped`, `stack.ungrouped`                                                                                                                                                                  | load, never fire; `compozy hooks list` warns                                                     | `window_manager.tab.split`, `tab.unsplit`                                                                  |
| Config `raise_on_focus`, `drag_away_policy`, `group_move_modifier`, `snap.*`, `bindings.top_center`, `bindings.bottom_center`                                                                                            | accepted, ignored, one warning                                                                   | remove the key                                                                                             |
| Config `new_window_policy = "floating"`                                                                                                                                                                                  | loaded as `"tab"` with a warning                                                                 | `"tab"`                                                                                                    |
| Shortcut IDs `window.minimize`, `window.zoom`, `window.toggle_floating`, `window.tile.top-left`, `window.tile.top-right`, `window.tile.bottom-left`, `window.tile.bottom-right`, `window.merge_all`, `window.tab.detach` | bindings load and do nothing; Settings marks them retired                                        | remove the binding (`window.unsplit` replaces `window.tab.detach`)                                         |

Results that embed topology (`Snapshot`, `Desktop`, `Window`, layout documents, closed entries) have
the version 6 shape with no version 5 projection.

## Reserved Partitions

Each profile has two reserved desktop partitions besides its per-project partitions:

- `global` is the **All projects** desktop. The Web binds it while the topbar globe is on. It is
  operator UX state: every agent identity, including agents with the unrestricted grant, is denied.
  Window, desktop, and layout tools accept `workspace: "global"` for operator calls only, before
  registry normalization; other native tools reject `global`. HTTP/UDS reach it at
  `/api/workspaces/global/window-manager`; CLI `--workspace global` is a data selector and fails with
  `workspace_reserved`.
- `user` is the **User** desktop, bound when the User destination is focused. Public WM paths and
  native tools accept `workspace: "@user"` and map it to the acting profile's `user` partition.
  Granted agents may use their own profile's `user` partition and project partitions; another
  profile's partitions stay operator-only.

Window data binding lives in `route.search.workspace`. A serialized binding always wins in every
partition: a registered workspace ID reads that project, `"@user"` reads User. An unbound window
reads its partition: `global` reads the aggregate (catalog windows), a project partition reads that
project, `user` reads User. In `global`, open, navigate, reopen, and layout replacement reject a
non-empty binding that is neither `@user` nor a registered workspace with HTTP 400
`window_binding_invalid`. Project partitions are not validated, so existing layouts stay valid.
Snapshot validation is registry-free: a saved layout always loads, and stale bindings are handled by
reconciliation.

Session deletion reconciles every live and persisted partition of every profile (project, `global`,
`user`): windows on the deleted session move to the empty sessions view, and closed entries,
navigation history, and undo/redo for it are dropped. Workspace unregistration retires windows
bound to that workspace onto their app's catalog route in every partition and removes the binding
from closed entries, navigation, and undo/redo; nothing brings it back, and a re-registered folder is
a new workspace ID. Each reconciliation publishes the partition's normal window-manager event.

Layouts of the former `COMPOZY_HOME` pseudo-project are copied to the profile's `user` partition
during the v0.4.0 upgrade when the target is absent; an existing User layout wins and the source
snapshot is kept under its original key.
