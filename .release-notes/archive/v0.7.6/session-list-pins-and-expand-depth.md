---
title: Pin sessions above the session list and choose how far to expand it
type: feature
---

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
