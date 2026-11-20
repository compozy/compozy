---
title: Ask about a running session in a side chat
type: feature
---

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
