## 0.7.7 - 2026-11-20

### Bug Fixes

- Resolve always-on Loop worker creation identity (#1855)
- Identify missing skills in native tool responses (#1866)
- Make table-rebuild migrations crash-safe (#1877)
- Bring the session barrel back under the source-size limit (#1879)
- Stop the MCP stdio crash helper from running the daemon test bootstrap (#1884)
- Refresh daemon provider readiness after CLI login (#1883)
- Keep session reply watches alive through follow-up work (#1878)
- Speed up session derivation and expose inherited transcripts (#1892)
- Make Fork and Continue fast to leave and show what the child inherited (#1888)
- Complete Antigravity file tool calls as they finish (#1889)
- Enforce queued daily budgets and repair release checks
- Preserve session derive and usage contracts

### Features

- Redesign the MCP OAuth callback page (#1860)
- Carry pinned subtrees and order Pinned manually (#1861)
- Add an opt-in budget for hosted MCP tool listings (#1870)
- Add side chats to CompozyOS sessions (#1867)
- Add permanent removal and restore for built-in extensions (#1862)
- Focus the composer when a session is opened (#1876)
- Add daily spend guardrails for always-on instances (#1849)

### Performance Improvements

- Stop rebuilding MCP projections on idle refresh ticks (#1871)
- Reduce repeated daemon session work (#1875)

### Testing

- Include pin_position in the sessions schema contract (#1863)
- Age the work-log triage instance after the shared DB clock stamps it

### Release Notes

#### Features

##### Remove built-in extensions permanently and restore them

Removing a built-in extension, such as `spec-cycle`, used to last only until the next daemon start,
when CompozyOS enrolled it again. You can now remove a built-in permanently, and it stays removed
across restarts until you restore it. Use **Remove permanently** in the remove dialog, or run
`compozy extension remove <name> --permanent`. Over HTTP and UDS, send
`DELETE /api/extensions/<name>?permanent=true`. Permanent removal needs operator access. Plain
`remove` works as before, and the built-in returns at the next start.

A permanent removal deletes the same things a plain removal does. Specs and workspace files are not
touched, and the extension's MCP overrides and input values are kept.

To bring a built-in back, use **Restore** in the **Removed built-ins** section of the installed
list, which appears while at least one built-in is permanently removed. From the command line,
`compozy extension restore <name>` restores it and `compozy extension removed [-o json]` lists the
removed built-ins. The HTTP and UDS routes are `POST /api/extensions/<name>/restore` and
`GET /api/extensions/removed-builtins`. Agents can list and restore removed built-ins with
`compozy__extensions_removed_list` and `compozy__extensions_restore`, but no tool removes a built-in
permanently.

If a restore fails, the built-in stays removed and the call returns `extension_restore_failed`. An
install that conflicts with an existing extension still returns 409 and now carries the
`extension_exists` code.

##### The prompt is ready to type in when you open a session

Opening a session now puts the caret in its prompt, so you can start typing without clicking first.
This happens when you open a session on purpose: clicking its row or pressing Enter on it, choosing
it in the command palette, the dock, a notification, or the attention dock, opening a board card, a
session you just created, and following a link or going Back or Forward to a session. It happens
only when the session can take a prompt.

Focus stays where it is when you step through sessions with Previous and Next session, when a
dialog, menu, or another text field such as a terminal or the Files editor has focus, while you edit
a message inline, when the session window is not the focused window, and when you click somewhere
else right after opening the session.

##### Pinned sessions keep their children and your order

Pinning a session now carries its child sessions with it. In **Pinned**, the children sit nested
under the pinned session at every depth, and they leave their project group and folder while the
pin lasts, so nothing shows twice and group and folder counts leave them out. **Unpin** returns the
session and its children to the places they held. Forks, and continuations of the pinned session
itself, are new top-level work and stay where they are.

You can now order Pinned yourself. A new pin goes on top; drag a pin up or down, or use **Move up**
and **Move down** in its **Move** menu. The order is the same after a reload and in every window.
Reordering works where the list shows one project and profile, and is off while you search. From
the command line, `compozy session reorder <id>... --pinned` sets the order and
`compozy__session_reorder` takes `pinned: true`.

`compozy session list --pinned only|exclude`, the sessions API `pinned` filter, and
`compozy__session_list` now include or leave out the sessions a pin carries along with the pin.
`sort=pinned` follows your Pinned order and pages by pin: each page holds whole subtrees, and its
total counts pins.

## Migration notes

Existing pins keep their order on upgrade: the oldest pin stays first, and new pins go above them.
A `sort=pinned` cursor from a previous version is refused once; read the first page again.

##### Ask about a running session in a side chat

You can now ask a question about a session's work while the session keeps going. Type
`/side <question>` in the composer, press ⌘⌥S (Ctrl+Alt+S off macOS), or use the **Side chats**
toggle in the session head. The side chat opens in a resizable panel in the session window, in the
same place as the subagent drawer, and nothing is added to the main thread. A bare `/side` opens an
empty side chat.

A side chat is a separate session that starts from its source's conversation. When the source is
idle, the side chat starts as a copy of it. When the source is working, it starts from a bounded
replay of the history that includes the turn in progress. Its first prompt tells the agent to answer
questions and not to continue the main task. It uses the source's runtime and permissions, and its
composer has its own runtime picker. Nothing flows back to the source: no message, no wake, and no
input event.

Side chats are listed only under their source, never in the session list, board, search, or
palette. The panel header has a switcher for the session's side chats, a chip that shows whether the
main session is still working or finished, and **Close side chat** and **Hide side chats**. The
toggle shows how many side chats are open and an attention dot when one needs you.

A side chat follows its source. When you stop the source, its side chats stop too, and archiving,
unarchiving, or deleting the source does the same to them. The delete dialog says so. History
cleanup removes a source's side chats with it, and moving a session to another profile archives its
side chats instead of moving them. Closing a side chat archives it; side chats do not expire. A side
chat can't be forked, continued, moved, reparented, bound to a worktree, or put in Later. It can't
run Goals, spawn or delegate to agents, or open its own side chats. Each refusal returns its own
error code.

From the command line, `compozy session side open <id> <message...>` opens a side chat and
`compozy session side list <id> [--archived]` lists them. Over HTTP and UDS, use
`POST /api/workspaces/{workspace_id}/sessions/{session_id}/sides` and `GET .../sides`, with
`?archived=true` to include closed ones. Agents use `compozy__session_side_open` and
`compozy__session_side_list`. The catalog stream sends `session_side_changed`, hook
`SessionContext` gains the optional `lineage_kind` and `parent_session_id` fields, and the shortcut
action is `session.side.toggle`.

The composer's busy row now shows only **Stop**, which cancels the turn, in the main session and
in side chats. The **Queue**, **Steer**, and **Interrupt** buttons are gone. Enter still steers or
queues by default, and ⌘⏎ (Ctrl+⏎ off macOS) does the other. Interrupt is no longer a web action; it
remains available as `compozy session prompt --interrupt`, over HTTP and UDS, and in the native
tools.

#### Fixes

##### SQLite upgrades resume safely after an interruption

If CompozyOS stops while rebuilding SQLite tables during an upgrade, restarting it retries the
interrupted migration with your session catalog and derivation history intact.
