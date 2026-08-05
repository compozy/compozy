# Window Management

## Contents

- Operating model
- Tabs and stacks
- CLI
- Native tools
- HTTP, UDS, and stream
- Raw layout recovery
- Configuration and hooks

Compozy manages persistent virtual desktops, tiled groups, tab stacks, floating windows, and layout
history as one daemon-authoritative workspace topology. Never edit internal client-state keys or
invent window fractions. Use semantic desktop, window, and layout operations; the daemon validates,
normalizes, and commits the complete topology atomically.

## Operating Model

- A window belongs to exactly one persistent desktop and to exactly one container: the tiled group
  tree, the floating list, or a floating tab stack.
- Tiled groups contain `leaf`, weighted `split`, or `stack` nodes. Floating windows retain
  normalized geometry so clients can project the same topology into different viewports.
- Every durable mutation is compare-and-swap guarded by the workspace `revision`. A successful
  mutation advances it once and records one event. Layout/topology changes add one history entry;
  route navigation and stack activation do not enter or clear layout history.
- `desktop.switch`, `window.focus`, and zoom presentation require an explicit connected `client_id`.
  They never select an implicit foreground browser.
- A focus desktop exists only while its owner remains on it. When the owner returns, moves, swaps,
  minimizes, closes, or leaves through layout replacement, the daemon deletes an empty focus desktop
  or converts an occupied one to a standard desktop.
- Preview, validation, rejected commands, and no-ops do not persist or emit topology events.
- Snapshots and layout documents are version 3. The wire snapshot carries no `history` body; undo and
  redo stay daemon-internal, and the snapshot reports `closed_entry_count` instead of reopen bodies.

Read the current revision before mutating:

```bash
compozy layout get --workspace <workspace-id> -o json
```

Pass that value as `--revision` to each CLI mutation. On
`window_manager_revision_conflict`, read again and recompute the intended semantic command. Do not
blindly replay stale geometry.

## Tabs And Stacks

A tab is a full window that shares a frame with its siblings. One concept, two containers:

- A tiled tab frame is a `stack` node inside a desktop group tree.
- A floating tab frame is an entry in that desktop's `floating_stacks` (`id`, ordered `window_ids`,
  durable `active_id`, frame `rect`, `minimized`).

Stack rules the daemon enforces:

- A stack holds at least two members and one member `active_id`. Normalization dissolves a
  one-member stack back to a tiled leaf or a floating window at the frame rect.
- Pinned members form a contiguous prefix of `window_ids`. Reorder and group clamp an index into the
  member's own region, so pinned tabs never fall behind unpinned ones.
- `active_id` is the durable last-active member. Each connected client also keeps `stack_active`
  (stack id → window id) in its client view; that projection is presentation state, repaired on
  reconnect, and never replaces the durable value.

Per-window tab state:

- `nav_stack` holds the window's route ancestors, oldest first; `route` stays the current leaf.
- `pinned` survives grouping, ungrouping, and reopen.

Closing and reopening:

- Every non-minimize close pushes exactly one closed entry, whatever the scope: one command, one
  revision, one entry. Minimizing pushes none.
- `window reopen` pops the newest entry and recreates its windows with their original IDs, routes,
  nav stacks, and pins. Members still live are skipped. The entry rejoins its original stack when
  that stack still exists; otherwise a multi-window entry rebuilds a floating tab frame at the stored
  rect, and a single survivor returns as a floating window. A deleted desktop resolves to the bound
  client's active desktop, else the first standard desktop. An empty reopen history succeeds without
  applying.
- Window IDs are reserved across live windows **and** windows retained in closed entries. Reusing a
  reserved ID fails with `window_manager_invalid_command`; omit the ID and the daemon generates a
  free opaque one, readable from the command result's changed window IDs. Never parse a window ID.

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

Deleting the final desktop is rejected. Deleting a non-empty desktop requires a distinct explicit
destination so the transfer and deletion remain one transaction.

Client-local presentation:

```bash
compozy desktop clients register --workspace <workspace-id> --client <stable-client-id> -o json
compozy desktop switch --workspace <workspace-id> --revision <revision> \
  --client <stable-client-id> --id <desktop-id>
compozy window focus --workspace <workspace-id> --revision <revision> \
  --client <stable-client-id> --direction right
compozy window zoom --workspace <workspace-id> --revision <revision> \
  --client <stable-client-id> --id <window-id>
```

Use `compozy desktop clients list|register|unregister` to manage live presentation identities. Browser
clients persist only their random stable client ID locally; topology and revisions stay in the daemon.
Each client view advances its own monotonic `presentation_revision`, independently of the workspace
topology revision. Re-registering an existing stable ID without an active desktop preserves its
repaired view and is a no-op when nothing changed.

Window lifecycle and placement:

```bash
compozy window list --workspace <workspace-id> -o json
compozy window open --workspace <workspace-id> --revision <revision> --app tasks \
  --pathname /tasks --search-json '{}'
compozy window navigate --workspace <workspace-id> --revision <revision> --id <window-id> \
  --pathname /tasks/<task-id> --search-json '{"tab":"runs"}'
compozy window move --workspace <workspace-id> --revision <revision> --id <window-id> \
  --desktop <desktop-id> --target <window-id> --placement right
compozy window move --workspace <workspace-id> --revision <revision> --id <window-id> \
  --desktop <desktop-id> --group
compozy window float --workspace <workspace-id> --revision <revision> --id <window-id>
compozy window resize --workspace <workspace-id> --revision <revision> --id <window-id> \
  --rect 0,0,0.5,0.6
compozy window swap --workspace <workspace-id> --revision <revision> \
  --first <window-id> --second <window-id>
compozy window close --workspace <workspace-id> --revision <revision> --id <window-id> --minimize
compozy window open --workspace <workspace-id> --revision <revision> --restore <window-id>
```

Tabs:

```bash
compozy window group --workspace <workspace-id> --revision <revision> \
  --target <window-id> --windows <window-b>,<window-c> --insert-index 1
compozy window activate <window-id> --workspace <workspace-id> --revision <revision> \
  --client <stable-client-id>
compozy window pin <window-id> --workspace <workspace-id> --revision <revision>
compozy window unpin <window-id> --workspace <workspace-id> --revision <revision>
compozy window reopen --workspace <workspace-id> --revision <revision>
compozy window open --workspace <workspace-id> --revision <revision> --app tasks \
  --pathname /tasks --stack-target <window-id>
compozy window navigate --workspace <workspace-id> --revision <revision> --id <window-id> \
  --mode push --pathname /tasks/<task-id>
compozy window navigate --workspace <workspace-id> --revision <revision> --id <window-id> --mode pop
compozy window close --workspace <workspace-id> --revision <revision> --id <window-id> --scope others
```

- `group` takes one `--target` (the destination frame) and a comma-separated `--windows` list of
  distinct joiners, each different from the target. Members join in list order, the last one becomes
  active, and a solo target is converted into a stack first. `--insert-index` is the anchor in the
  target's member list, clamped into each joiner's pin region; omit it to append.
- `activate` sets the stack's durable `active_id`. With `--client` it also sets that client's active
  tab. It is the public name of the internal command `window.stack.set_active`.
- `pin` and `unpin` set the flag and re-collate the stack's pinned prefix. Pinning a non-stacked
  window is legal; the flag applies when it later joins a stack.
- `reopen` takes no target — it always pops the newest closed entry.
- `open --stack-target <window-id>` opens the new window as a tab of that window's frame and makes it
  active. It is exclusive with `--restore`.
- `navigate --mode replace|push|pop` (default `replace`). `push` records the current route in
  `nav_stack` before moving; `pop` returns to the newest `nav_stack` entry and rejects `--pathname`
  and `--search-json`. Popping an empty stack succeeds without applying.
- `close --scope tab|group|others|right` (default `tab`). `tab` refuses a pinned window; `group`
  closes every member of the frame, pinned included; `others` and `right` close only unpinned members
  of that stack. `--minimize` is legal only with `--scope tab`. On a non-stacked window, `others` and
  `right` succeed without applying.

`window list` reports `stack_id`, `member_order`, `active`, `pinned`, and `nav_depth` alongside id,
app, desktop, placement, and pathname, in every output mode.

Structural placements are `before`, `after`, `left`, `right`, `top`, `bottom`, and `center`;
`floating` accepts a normalized `x,y,width,height` rect. Dropping onto an occupied target reflows the
tree. When viewport minima cannot fit, clients project the affected split as a stack without
corrupting the durable topology. `--group` operates on the whole unit: with `--target` and a
structural placement the tab frame splices in beside it as one node (`center` folds the members into
the target's stack), with `--rect` the frame floats out as one unit at that rect, and with only
`--desktop` it relocates the frame or the source tiled group.

`placement` is structural (`tiled`, `stacked`, or `floating`); `route` is the durable internal
pathname plus canonical JSON-object search state. `window navigate` may include `--client` to switch
and focus that explicit connected client atomically. Without it, navigation changes no presentation.
With a bound client, explicit `window focus --id` and `window open --restore` also activate the
window's owning desktop for that client; `window swap` exchanges two units' structural places — a
stacked window swaps as its whole tab frame, and two members of one frame never swap. The browser
exposes the same command by dropping a drag on an occupied window's center, or anywhere over it
while `window_manager.swap_modifier` is held.

Layout operations:

```bash
compozy layout arrange --workspace <workspace-id> --revision <revision> --desktop <desktop-id> \
  --window <window-a> --window <window-b> --arrangement horizontal
compozy layout arrange --workspace <workspace-id> --revision <revision> --resource <resource-id>
compozy layout resize --workspace <workspace-id> --revision <revision> \
  --split <node-id> --boundary 0 --delta 0.05
compozy layout frame-resize --workspace <workspace-id> --revision <revision> \
  --desktop <desktop-id> --edit <group-a>=0,0,0.6,1 --edit <group-b>=0.6,0,0.4,1
compozy layout balance --workspace <workspace-id> --revision <revision> --group <group-id>
compozy layout undo --workspace <workspace-id> --revision <revision>
compozy layout redo --workspace <workspace-id> --revision <revision>
compozy layout watch --workspace <workspace-id> -o jsonl
compozy layout watch --workspace <workspace-id> --client <stable-client-id> -o jsonl
```

`layout resize` moves one split boundary in weight space. `layout frame-resize` atomically rewrites
abutting island frames: every group edge on the shared line moves together, and overlapping frames
are rejected. `window resize` assigns a normalized frame to the unit containing the window —
floating windows and floating tab frames take the rect directly, a solo tiled island resizes in
place, and a split member detaches into its own island at that frame while siblings keep their
exact zones. In the browser every shared boundary — split or island — is one draggable seam, and a
tiled unit's free edges and corners resize that unit alone, stopping at the nearest island.

`--resource` is exclusive with inline arrangement fields. Declarative `window_layout` resources are
data-only topology templates: strict, versioned, workspace-bound when scoped locally, with `any`,
`landscape`, or `portrait` aspect variants plus explicit participant slots and overflow policy. A
resource cannot execute code or receive pointer events. Discover them through
`compozy__resources_list`; they may be global or workspace-scoped, and a workspace resource wins when
IDs collide. Applying one runs the same preview, revision/CAS, validation, commit, event, and
history pipeline as an inline arrange command.

## Native Tools

The lazy `compozy__window_manager` toolset provides desktop, window, and layout operations with the same
workspace, revision, client, diagnostic, and risk contracts as the CLI/API. Resolve
`compozy__tool_info` for an exact tool before calling it. Use `window_manager.read` for reads/previews and
`window_manager.write` for mutations.

Tab tools mirror the CLI verbs one to one: `compozy__window_group`, `compozy__window_reorder`,
`compozy__window_activate`, `compozy__window_pin`, `compozy__window_reopen`. `window_reorder` is the
tool form of moving a member inside its own stack (`window_id` + clamped `index`); the CLI reaches the
same reordering through `window group --insert-index`. Three existing tools carry the tab inputs:
`compozy__window_open` accepts `stack_target_window_id`, `compozy__window_navigate` accepts
`mode` (`replace`/`push`/`pop`, and rejects `route` when `mode` is `pop`), and
`compozy__window_close` accepts `scope` (`tab`/`group`/`others`/`right`, rejected together with
`minimize`). All five tab tools are mutating and require `window_manager.write`.

## HTTP, UDS, And Stream

HTTP and UDS expose identical workspace routes under:

```text
/api/workspaces/{workspace_id}/window-manager
```

The surface includes snapshot, preview, commands, client list/register/unregister, layout
export/validate/apply, layout profiles, and the WebSocket stream. Tabs add no routes. The commands
and preview endpoints accept five more command IDs — `window.stack.group`, `window.stack.reorder`,
`window.stack.set_active`, `window.pin`, `window.reopen` — plus the extended `window.open`
(`stack_target_window_id` inside `window`), `window.navigate` (`mode`), and `window.close` (`scope`) payloads.
Payload decoding is strict: unknown fields are rejected, `mode: "pop"` rejects a non-empty `route`,
and `minimize` rejects a non-empty `scope`. The snapshot carries `floating_stacks` per desktop,
`nav_stack` and `pinned` per window, and `closed_entry_count`; client frames carry `stack_active`.

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
workspace's records.

Pass `--client <stable-client-id>` in the CLI, or an optional registered `client_id` over HTTP/UDS,
to bind the stream to one presentation view. Its initial
snapshot contains that client fence; later client frames carry only that ID and a strictly newer
`presentation_revision`, while topology event frames follow the workspace revision. An unbound
stream receives topology only. Never apply an equal or older client frame. If a subscriber is
evicted as slow, reconnect and replace local state from the next fence. If the daemon lost the
transient client registration, register the same stable ID before reconnecting. Never merge a
reconnect snapshot with an unfenced local mirror.

## Raw Layout Recovery

Semantic commands are the normal path. Raw layout replacement is the validated recovery escape
hatch:

```bash
compozy layout export --workspace <workspace-id> -o json > layout.json
compozy layout validate --workspace <workspace-id> --file layout.json -o json
compozy layout apply --workspace <workspace-id> --revision <revision> --file layout.json -o json
```

Documents are version 3 and round-trip `floating_stacks`, `nav_stack`, and `pinned`. Export omits
history and closed entries, so a raw round trip never restores reopen history. A version other than 3
fails validation with `window_manager_invalid_topology` and the `topology.unsupported_version`
diagnostic; there is no converter — rebuild the layout with semantic commands instead.

Validate never writes. Apply binds the document to the requested workspace,
validates the complete topology, preserves current routes for surviving window IDs, and performs one
atomic replacement. There is no raw key-value fallback.

A tiled return anchor in an exported document carries `return_anchor.source_group`, the daemon's
validated deep capture of its source group. For Zoom, an unchanged live source residue lets unzoom
restore that exact group identity, node order, weights, placement, and active stack member. If the
source changed while the window was zoomed, Compozy keeps those edits and uses the structural anchor
fallback. Treat `source_group` as daemon-owned recovery state: preserve it during raw document round
trips, and use `compozy window zoom` or `compozy__window_zoom` instead of fabricating or editing it.

## Configuration And Hooks

Global defaults live under `[window_manager]` with nested `gaps`, `snap`, `bindings`, and `shortcuts`.
Validated config updates hot-apply atomically; an invalid update leaves the active defaults unchanged.
Use `GET/PATCH /api/settings/window-manager` for the typed Settings surface. Edge-center bindings are
`zoom`, `reserved`, or `none`; any/landscape/portrait profiles are `window_layout` resources.
Workspace layout documents may carry typed overrides without changing other workspaces.

`nav_stack_limit` (default 50, range 1..200) caps each window's `nav_stack`; `closed_entry_limit`
(default 20, range 1..100) caps retained closed entries. Both are write-time caps resolved from the
effective workspace config when the mutation runs, evicting the oldest value. Lowering a live limit
is not retroactive: it applies to the next relevant mutation, not to stored state.

Committed operations emit nine async hooks. `window_manager.layout.applied`,
`window_manager.desktop.created`, `window_manager.desktop.deleted`, and
`window_manager.window.moved` follow their commands. Tab operations add
`window_manager.window.opened` (`window.open`, `window.reopen`),
`window_manager.window.closed` (every close scope, carrying all removed window IDs),
`window_manager.stack.activated` (durable activation via `window.stack.set_active` and the daemon's
coalesced last-active commits), `window_manager.stack.grouped` (a stack formed or extended by
`window.stack.group`, `window.open --stack-target`, or a reopen that rebuilds a frame), and
`window_manager.stack.ungrouped` (a stack dissolved by close, by a group that empties its source
stack, or by floating a member out). Preview, pointer motion, navigation, client focus, per-client
tab flips, and desktop switching emit no extension hooks.

Stable public errors include `window_manager_revision_conflict`,
`window_manager_invalid_topology`, `window_manager_destination_required`,
`window_manager_history_boundary`, `window_manager_slow_consumer`, `window_manager_not_stacked`
(a stack operation on a window that is in no stack), and `window_manager_window_pinned`
(`close --scope tab` on a pinned window — unpin it or use `--scope group`). Preserve structured
diagnostics and entity conflicts when reporting failures.
