# Window Management

## Contents

- Operating model
- CLI
- Native tools
- HTTP, UDS, and stream
- Raw layout recovery
- Configuration and hooks

AGH manages persistent virtual desktops, tiled groups, floating windows, and layout history as one
daemon-authoritative workspace topology. Never edit internal client-state keys or invent window
fractions. Use semantic desktop, window, and layout operations; the daemon validates, normalizes,
and commits the complete topology atomically.

## Operating Model

- A window belongs to exactly one persistent desktop.
- Tiled groups contain `leaf`, weighted `split`, or `stack` nodes. Floating windows retain
  normalized geometry so clients can project the same topology into different viewports.
- Every durable mutation is compare-and-swap guarded by the workspace `revision`. A successful
  mutation advances it once and records one event. Layout/topology changes add one history entry;
  route navigation does not enter or clear layout history.
- `desktop.switch`, `window.focus`, and zoom presentation require an explicit connected `client_id`.
  They never select an implicit foreground browser.
- Preview, validation, rejected commands, and no-ops do not persist or emit topology events.

Read the current revision before mutating:

```bash
agh layout get --workspace <workspace-id> -o json
```

Pass that value as `--revision` to each CLI mutation. On
`window_manager_revision_conflict`, read again and recompute the intended semantic command. Do not
blindly replay stale geometry.

## CLI

Desktop lifecycle:

```bash
agh desktop list --workspace <workspace-id> -o json
agh desktop create --workspace <workspace-id> --revision <revision> --name Build -o json
agh desktop update --workspace <workspace-id> --revision <revision> --id <desktop-id> --name Review
agh desktop reorder --workspace <workspace-id> --revision <revision> --id <desktop-id> --order 0
agh desktop delete --workspace <workspace-id> --revision <revision> --id <desktop-id> \
  --destination <desktop-id>
```

Deleting the final desktop is rejected. Deleting a non-empty desktop requires a distinct explicit
destination so the transfer and deletion remain one transaction.

Client-local presentation:

```bash
agh desktop clients register --workspace <workspace-id> --client <stable-client-id> -o json
agh desktop switch --workspace <workspace-id> --revision <revision> \
  --client <stable-client-id> --id <desktop-id>
agh window focus --workspace <workspace-id> --revision <revision> \
  --client <stable-client-id> --direction right
agh window zoom --workspace <workspace-id> --revision <revision> \
  --client <stable-client-id> --id <window-id>
```

Use `agh desktop clients list|register|unregister` to manage live presentation identities. Browser
clients persist only their random stable client ID locally; topology and revisions stay in the daemon.
Each client view advances its own monotonic `presentation_revision`, independently of the workspace
topology revision. Re-registering an existing stable ID without an active desktop preserves its
repaired view and is a no-op when nothing changed.

Window lifecycle and placement:

```bash
agh window list --workspace <workspace-id> -o json
agh window open --workspace <workspace-id> --revision <revision> --app tasks \
  --pathname /tasks --search-json '{}'
agh window navigate --workspace <workspace-id> --revision <revision> --id <window-id> \
  --pathname /tasks/<task-id> --search-json '{"tab":"runs"}'
agh window move --workspace <workspace-id> --revision <revision> --id <window-id> \
  --desktop <desktop-id> --target <window-id> --placement right
agh window move --workspace <workspace-id> --revision <revision> --id <window-id> \
  --desktop <desktop-id> --group
agh window float --workspace <workspace-id> --revision <revision> --id <window-id>
agh window swap --workspace <workspace-id> --revision <revision> \
  --first <window-id> --second <window-id>
agh window close --workspace <workspace-id> --revision <revision> --id <window-id> --minimize
agh window open --workspace <workspace-id> --revision <revision> --restore <window-id>
```

Structural placements are `before`, `after`, `left`, `right`, `top`, `bottom`, and `center`;
`floating` accepts a normalized `x,y,width,height` rect. Dropping onto an occupied target reflows the
tree. When viewport minima cannot fit, clients project the affected split as a stack without
corrupting the durable topology. Group relocation moves the source window's tiled group and is
exclusive with `--target`, `--placement`, and `--rect`.

`placement` is structural (`tiled`, `stacked`, or `floating`); `route` is the durable internal
pathname plus canonical JSON-object search state. `window navigate` may include `--client` to switch
and focus that explicit connected client atomically. Without it, navigation changes no presentation.
With a bound client, explicit `window focus --id` and `window open --restore` also activate the
window's owning desktop for that client; `window swap` exchanges two windows' structural places
(the browser exposes the same command as a drag-drop while `window_manager.swap_modifier` is held).

Layout operations:

```bash
agh layout arrange --workspace <workspace-id> --revision <revision> --desktop <desktop-id> \
  --window <window-a> --window <window-b> --arrangement horizontal
agh layout arrange --workspace <workspace-id> --revision <revision> --resource <resource-id>
agh layout resize --workspace <workspace-id> --revision <revision> \
  --split <node-id> --boundary 0 --delta 0.05
agh layout balance --workspace <workspace-id> --revision <revision> --group <group-id>
agh layout undo --workspace <workspace-id> --revision <revision>
agh layout redo --workspace <workspace-id> --revision <revision>
agh layout watch --workspace <workspace-id> -o jsonl
agh layout watch --workspace <workspace-id> --client <stable-client-id> -o jsonl
```

`--resource` is exclusive with inline arrangement fields. Declarative `window_layout` resources are
discovered through `agh__resources_list` and may be global or workspace-scoped; a workspace resource
wins when IDs collide.

## Native Tools

The lazy `agh__window_manager` toolset provides desktop, window, and layout operations with the same
workspace, revision, client, diagnostic, and risk contracts as the CLI/API. Resolve
`agh__tool_info` for an exact tool before calling it. Use `window_manager.read` for reads/previews and
`window_manager.write` for mutations.

## HTTP, UDS, And Stream

HTTP and UDS expose identical workspace routes under:

```text
/api/workspaces/{workspace_id}/window-manager
```

The surface includes snapshot, preview, commands, client list/register/unregister, layout
export/validate/apply, layout profiles, and the WebSocket stream. Layout profile records use
`GET /layout-profiles` and `PUT|DELETE /layout-profiles/{profile_id}` below the workspace route. A
list contains global records plus records scoped to that workspace; writes cannot address another
workspace. HTTP profile mutations require a loopback listener, while UDS exposes the same request
and response contract.

The CLI covers the same records:

```bash
agh layout-profile list --workspace <workspace-id> -o json
agh layout-profile get <profile-id> --workspace <workspace-id> -o json
agh layout-profile put <profile-id> --workspace <workspace-id> \
  --scope workspace|global --file profile.json --expected-version <n> -o json
agh layout-profile delete <profile-id> --workspace <workspace-id> --expected-version <n> -o json
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
agh layout export --workspace <workspace-id> -o json > layout.json
agh layout validate --workspace <workspace-id> --file layout.json -o json
agh layout apply --workspace <workspace-id> --revision <revision> --file layout.json -o json
```

Export omits history. Validate never writes. Apply binds the document to the requested workspace,
validates the complete topology, preserves current routes for surviving window IDs, and performs one
atomic replacement. There is no raw key-value fallback.

A tiled return anchor in an exported document carries `return_anchor.source_group`, the daemon's
validated deep capture of its source group. For Zoom, an unchanged live source residue lets unzoom
restore that exact group identity, node order, weights, placement, and active stack member. If the
source changed while the window was zoomed, AGH keeps those edits and uses the structural anchor
fallback. Treat `source_group` as daemon-owned recovery state: preserve it during raw document round
trips, and use `agh window zoom` or `agh__window_zoom` instead of fabricating or editing it.

## Configuration And Hooks

Global defaults live under `[window_manager]` with nested `gaps`, `snap`, `bindings`, and `shortcuts`.
Validated config updates hot-apply atomically; an invalid update leaves the active defaults unchanged.
Use `GET/PATCH /api/settings/window-manager` for the typed Settings surface. Edge-center bindings are
`zoom`, `reserved`, or `none`; any/landscape/portrait profiles are `window_layout` resources.
Workspace layout documents may carry typed overrides without changing other workspaces.

Committed operations emit exactly four async hooks: `window_manager.layout.applied`,
`window_manager.desktop.created`, `window_manager.desktop.deleted`, and
`window_manager.window.moved`. Preview, pointer motion, navigation, client focus, and desktop
switching do not emit extension hooks.

Stable public errors include `window_manager_revision_conflict`,
`window_manager_invalid_topology`, `window_manager_destination_required`,
`window_manager_history_boundary`, and `window_manager_slow_consumer`. Preserve structured
diagnostics and entity conflicts when reporting failures.
