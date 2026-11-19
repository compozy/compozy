## 0.7.6 - 2026-11-19

### Bug Fixes

- Wake pull request monitors for later check rounds (#1812)
- Exclude bot-maintained tracking issues from issue intake
- Restore unprompted always-on mains after restart (#1818)
- Accept slash-terminated MCP OAuth root resources (#1825)
- Renew MCP OAuth tokens for CIMD clients (#1833)
- Answer a pending clarification from the session composer (#1835)
- Return retryable refusals for hosted MCP binds (#1843)
- Accept the Claude Code plugin schema in Marketplace (#1842)
- Renew MCP OAuth grants before expiry (#1847)
- Keep Marketplace installs responsive during refresh (#1841)
- Support pre-registered OAuth MCP discovery (#1844)
- Keep hosted MCP sessions responsive with large catalogs (#1854)
- Restore release integration and E2E coverage (#1856)

### Features

- Add six-digit gateway pairing codes and device removal (#1819)
- Surface offline notices and automation failures in the attention bell (#1820)
- Add keyboard-first navigation to the desktop shell (#1832)
- Pin sessions above the session list (#1834)
- Choose session sidebar expansion depth (#1853)
- Turn configured MCP servers off and on (#1852)

### Release Notes

#### Features

##### See missed notices and failed automations in the attention bell

A notice sent with `compozy notify` or `compozy__notify` while no CompozyOS window is open is no
longer lost. The daemon stores it, and the sending profile sees it in the attention bell after it
reconnects, even after a daemon restart. Such a notice still reports `no-client`. Muted and
rate-limited notices are not stored.

Failed job and trigger runs now appear under **Needs you** in the attention bell, across profiles,
for User and registered projects. Select a failure to open its job or trigger without switching the
selected profile. Marking it as read records the acknowledgement for the current profile and
operator and leaves the run history unchanged.

##### Pair remote devices with a six-digit code and remove old devices

Pairing a device for remote access now leads with a six-digit code. **Settings → Gateway → Pair a
device** shows the code grouped for reading, such as `123 456`, and copies it as six digits. The QR
code, the pairing link and the long pairing artifact are still available as secondary options. The
dialog now fits narrow phone screens without scrolling sideways.

At the pairing gate on the new device, type the six-digit code, with or without spaces or dashes, or
paste the pairing link or artifact. `compozy pair mint` prints the six-digit code next to the path
of the private artifact file, and `compozy pair redeem` accepts either one. The code is single-use,
shares the artifact's five-minute expiry, and redeeming either form uses up both. Repeated wrong
codes are rate limited per source address.

You can now remove a paired device instead of leaving it in the list forever. Use **Remove** on the
device's row in Settings → Gateway, or run `compozy device remove <device-id>`. Removing an active
device revokes it first and closes its live streams. Removing a revoked device deletes only its
record. Each removal appends a `gateway.device.removed` event before the record is deleted, so the
audit history remains. Revoke works as before. Agents can remove a device through the
`device_remove` action of `compozy__gateway`, and the HTTP and UDS route is
`DELETE /api/gateway/devices/{id}/record`.

##### Drive the desktop shell from the keyboard

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
selection, and zoom. On macOS, an Option chord your keyboard layout turns into a character, such as
⌥L for `@` on a German layout, stays typing; a letter outside ASCII, such as `ł` on a Polish layout,
still runs the bound chord. On Linux and Windows, bound Alt chords leave the terminal, so shell Alt
bindings on those keys need a rebind. Holding a bound chord runs it once instead of typing the
character.

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
  gives that chord up. An extension's default chord that a new default now uses stops working until
  you bind it.
- **Terminal preset removed.** Its window, tab, and session chords are now the defaults, so Settings
  no longer offers the preset. Its attention (⌥I), tab-jump (⌃⌥1–9), workspace, and desktop chords did not
  become defaults. Values you applied earlier stay in your config and keep working. If you applied
  it, your stored Previous and Next session bindings replace the new defaults, so ⌃⌥↑/↓ do not walk
  sessions; reset those two rows in Settings › Shortcuts to get them.
- **Zoom on macOS.** Zoom in, out, and reset use ⌘ only; ⌃=, ⌃-, and ⌃0 no longer zoom.
- **Moved page.** Shortcuts moved from Settings › Layouts to Settings › Shortcuts
  (`/settings/shortcuts`). An old `/settings/layouts?command=<id>` link opens the Shortcuts page
  focused on that command until v0.9.0.

##### Pin sessions above the session list and choose how far to expand it

You can pin a session to keep it above everything else in the session list. Hover or focus a row
and use the pin button beside `…`, or choose **Pin to top** from the row's `…` menu or context menu.
Pinned sessions appear in a **Pinned** section at the top of the list, below search and above every
project group and folder. The oldest pin comes first. In All projects the section holds pins from
every project; a project's own list shows only its pins.

A pin moves only the session you chose. Its parent, children, siblings and folder stay where they
are, and folder and project counts drop the pinned row. A project whose only visible session is
pinned leaves the list until you unpin it. **Unpin** returns the session to the place it held,
including its spot in a manually ordered folder, and pinning does not change the session's last
activity time. A pin survives stop, archive, Later, reload and daemon restart. Archived and Later
sessions keep a pin mark with **Unpin** and return to Pinned when restored. Subagent sessions and
always-on agent main sessions cannot be pinned. Kanban is unchanged.

From the CLI, `compozy session pin <id>` and `compozy session unpin <id>` set and clear a pin, and
`compozy session list --pinned` lists pinned sessions. `compozy__session_list` gains a `pinned`
filter and returns `pinned_at`. Over HTTP and UDS, `POST` and `DELETE`
`/api/workspaces/{workspace_id}/sessions/{session_id}/pin` pin and unpin a session.

The expand button in the session-list header now opens a menu with three choices:

- **Expand groups** opens folders and project groups and keeps child-session lists folded, so no
  subagent list opens.
- **Expand all** opens everything, as the button did before.
- **Collapse all** folds everything.

The button's label and tooltip show the current choice. A child list you open by hand stays open
under Expand groups, and a search still shows matching child sessions. The Pinned section folds
under Collapse all and opens under both expand choices. The Kanban header keeps its single Collapse
all / Expand all button.

#### Fixes

##### Keep sessions responsive with large MCP catalogs and turn MCP servers off

Sessions stay responsive when your MCP servers expose large tool catalogs. Starting a session no
longer spends its launch budget discovering upstream catalogs: it binds to the catalog CompozyOS
already holds, and a background worker refreshes servers on its own. Idle sessions no longer list
tools again just to renew a cache, and sessions that see the same tools share one copy of the
catalog instead of each keeping its own. A server whose discovery fails keeps its last successful
tools and reports why it failed.

A server with a live `tools/list_changed` subscription refreshes when it reports a change. For a
server without one, CompozyOS runs one shared refresh per server and credential binding while an
authorized session is bound. The interval is at least 60 seconds, or the server's `ttlMs` hint if
that is longer, and two minutes when the server gives no hint. A failed refresh waits the same
interval and keeps the last good catalog. Refreshing stops when the last session using the server
ends. **Check again** still contacts the server immediately.

A stdio server is no longer treated as failing, and restarted, when the caller's own deadline ran
out. CompozyOS now times the server's response after it accepts the call. When a session bind is
slow, the daemon log shows how long each phase took.

You can now turn a configured MCP server off without deleting it. Use the switch on the server's
row in **Settings → MCP servers**, or the CLI:

```bash
compozy mcp disable example --scope workspace --workspace my-project
compozy mcp enable example --scope workspace --workspace my-project
```

The verbs take the same scope selectors as the other `compozy mcp` commands. For a server an
extension provides, pass `--owner extension:<installed-name>`. You can also set `disabled = true` in
a `[[mcp_servers]]` entry in `config.toml`, or `"disabled": true` in an `mcp.json` server object.
Over HTTP and UDS the operation is `PUT /api/settings/mcp-servers/{name}/enabled` with a body such as
`{"enabled":false}`.

An off server keeps its definition, credentials and approval. It contributes no tools, and
CompozyOS skips connecting to it, discovering its tools, checking it and renewing its credentials.
The session's **MCP servers** control lists it as **Off**. Open sessions pick up a change on their
next message. An off server declared in a narrower scope still takes precedence over a server with
the same name in a broader scope, so turning it off does not fall back to the broader one. Servers
with no configured value stay on, so existing configuration keeps working unchanged.

##### Keep MCP OAuth sign-ins working longer

MCP servers that use OAuth stay signed in more reliably.

- **Renewal for client metadata document sign-ins.** After you signed in to a remote MCP server that
  identifies CompozyOS with a client ID metadata document, an expired access token could not be
  renewed even though a refresh token was stored, and the server's tools disappeared. CompozyOS now
  renews these tokens. Existing sign-ins renew in place, with no new login, and Settings no longer
  asks you to sign in again for an expired token that can still renew.
- **Longer-lived grants.** When the authorization server advertises `offline_access` and the client
  allows it, CompozyOS now requests it at sign-in so the server can issue a refresh token. A
  response without a refresh token still signs you in.
- **Renewal before expiry.** CompozyOS renews a token during the last 10% of its lifetime, at most 30
  seconds early, instead of waiting until it expires. During a temporary network or server error it
  keeps using the current token until it actually expires.
- **No endless retries.** When the stored client registration can no longer work, for example an
  expired or missing client secret, CompozyOS stops retrying and asks you to sign in again. Temporary
  network, metadata and storage errors still retry with backoff.
- **Pre-registered OAuth clients.** Some servers that use a pre-registered OAuth client finished
  sign-in but never loaded their tools. CompozyOS now accepts granted scopes separated by commas as
  well as spaces, and accepts the `2025-06-18` protocol revision those servers negotiate over
  Streamable HTTP. When a server rejects the current token, the next check renews it if it can and
  otherwise asks you to sign in.
- **Clearer failures.** A server that cannot be reached now reports the HTTP status, JSON-RPC error
  or network failure in its status instead of a generic message, with credentials redacted. A
  stored credential that cannot be used now shows an authentication reason.

##### Composer answers to clarifications, faster Marketplace installs and more fixes

- **Answer an agent's question from the composer.** When an agent is waiting on a clarification,
  a message typed in the session composer now answers that question and the turn continues. Before,
  the turn stayed waiting until the question timed out. While a question is pending, the composer
  reads "Type here to answer the agent's question". A message with attached files is refused and
  your draft is kept. A question that only accepts one of its listed choices refuses a typed answer
  with a reason instead of dropping it, and you can still answer it by choosing an option.
- **Marketplace installs during refresh.** Opening an install preview no longer waits behind a
  catalog source refresh. Previously a preview could take minutes while sources refreshed.
- **Claude Code plugins in Marketplace.** Marketplace now installs plugins whose
  `.claude-plugin/plugin.json` uses the official Claude Code schema; the installer used to reject
  them. A package with an invalid manifest is now marked as not installable, and its detail page and
  install preview show why.
- **Retryable refusal when MCP tools fail to load.** If a session's MCP tools time out or fail to
  load while a message starts or restarts its runtime, the message is refused as not sent instead of
  failing with a server error. The composer restores your draft and shows "Message not sent", so you
  can send it again.
