## 0.6.3 - 2026-11-13

### Bug Fixes

- Download the recorded desktop release when the update feed is unreachable (#1643)
- Keep continued child sessions in their parent's session tree (#1647)
- Read the Claude Keychain login past a stale macOS credential file (#1648)
- Promote synthetic wakes with live ACP defaults (#1650)
- Count only closing PR references as issue-triage coverage

### Features

- Add a Move under parent picker for session organization (#1646)
- Group CompozyOS menus into submenus with List and Kanban parity (#1652)

### Performance Improvements

- Reuse the session writer's transcript encode buffer across appends (#1634)
- Read only triage rows in the periodic triage session sweep (#1636)
- Reload skill watcher config only after its files change (#1627)
- Share hosted MCP projection discovery across session streams (#1629)

### Testing

- Isolate the telemetry benchmark receiver process (#1639)

### Release Notes

#### Features

##### Choose a parent for a session from its menu or the command palette

Nesting a session under another used to need a drag, or a submenu that listed only the sessions already loaded in the sidebar.

- **Move under….** A session's row menu and right-click menu open a parent picker that searches the project's sessions by title or agent. Arrow keys and Enter choose the new parent, and the confirmation offers Undo.
- **From the command palette.** With a session window focused, type `move` or `parent` and choose "Move under another session…" to open the same picker.
- **Back to the top level.** On a nested session, the picker offers Make top-level, as the row menu does.
- **Continued and forked sessions stay put.** Their picker explains that they stay linked to the session they came from instead of listing parents. The daemon's refusal now says the same.

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

##### Continued child sessions stay under their parent in the session list

Continuing a child session no longer moves the new session to the top of the session list. When an orchestrator continues a worker, for example after a daemon restart stopped it, the continuation now appears in the orchestrator's thread next to the session it continues. This also holds when a session is continued more than once. Continuations of top-level sessions and forks still show as top-level sessions, and the continued session still names its source as "Continued from".

Only placement in the list changes. Permissions, lifecycle and child counts work as before: a continuation is not counted as one of the parent's children and does not stop when the parent stops. Session list and detail reads now include `tree_parent_session_id`, the parent a continuation is listed under. Sessions continued before this release move under their parent the next time the list loads; no migration is needed.

##### Desktop app updates download on networks that block the update feed

When the desktop app cannot reach its update feed, it now downloads the release the daemon already selected straight from the GitHub release and verifies it against the release's signed checksum. A failed update now says which network error stopped the download, such as `net::ERR_CONNECTION_CLOSED`, instead of "The app update could not be installed."

Migration notes: the fix ships in the updater itself. If updating from 0.6.0, 0.6.1, or 0.6.2 keeps failing with a connection error, download this release once from the GitHub release page and install it manually; later updates use the fallback automatically.

##### Hosted MCP sessions share tool discovery

CompozyOS reuses MCP tool discovery across open sessions with the same provider listing scope during each refresh window, reducing repeated discovery and descriptor processing. Sessions retain their own tool permissions and availability checks, and separate workspaces and profiles keep separate listings. Catalog changes retain the existing refresh bound.

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
