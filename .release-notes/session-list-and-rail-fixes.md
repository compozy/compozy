---
title: Session lists load again after a parent is deleted, and the rail keeps its place
type: fix
---

- **Workspace session lists after a deleted parent.** On v0.7.7, deleting or cleaning up a session
  whose children were still around could make every workspace-scoped session read fail with
  `session_not_found`. That broke `compozy session list --workspace`, `compozy worktree list`,
  `compozy worktree create` and `compozy session new --workspace`, and such children silently
  dropped out of the sidebar. These reads work again, and those children show in the sidebar.
- **Children of deleted side chats stay hidden.** When you delete a side chat while a child it
  started, such as a subagent, keeps running, that child no longer shows up as a top-level session
  in the session list, the catalog or `compozy__session_list`. You can still read it by ID.
- **The rail keeps its scroll when you switch sessions.** Clicking a row below the fold in a
  Session window's rail no longer sends the list back to the top. The rail also returns to where you
  were when its list finishes loading after the switch, until you scroll it yourself. While the
  picked session loads, the real rail stays beside the loading conversation instead of placeholder
  rows. A reload still starts at the top.
- **Notifications reuse your Session window.** Clicking a toast, an OS notification, or a session or
  notice row in the attention bell no longer opens a new Session tab for every session. It lands in
  your Session window by the same rule that new sessions use, and a session that already has a window
  is focused there. The rail scrolls the session into view.
- **Folders no longer push other sessions off the first page.** A folder with many newer sessions
  could take the whole first page of the list and hide the sessions outside folders. The sessions
  outside folders and each folder now page on their own, each with its own Previous and Next
  controls and its own total. Archived and Later keep their Manual order. Over HTTP and UDS, the
  sessions list accepts an optional `folder=<id>` or `folder=@root` selector, which needs one
  project and one profile; leaving it out returns the whole list as before.
- **Global always-on agents appear once.** When the session list or the Kanban board is grouped by
  project, Global always-on agents now show only in the **Global** row, not again in the **User**
  group.
- **Group totals leave out always-on main sessions.** A quiet Global always-on agent could make the
  User group read one session with no rows, and project totals counted agent main sessions too.
  Requests that send `exclude_instance_main=true`, as the List and Kanban views do, now leave those
  sessions out of the counts as well. Requests without the flag count as before.
