---
title: Agent editors ask before a window command discards unsaved edits
type: fix
---

Unsaved edits in the agent editors (Instructions, Heartbeat, Memory, the authored SOUL and HEARTBEAT files, Permissions and the Tools panel) are no longer lost when the window manager takes the editor away. Closing the window or tab with Cmd+W, a close from the menu, or a bulk close now asks first; a bulk close asks once for every affected window. Navigation started from outside the window, such as `compozy window navigate` or a project switch, keeps the draft in memory instead: the editor shows it as unsaved when you return, and saving it goes through the usual conflict check.

Migration notes: no action needed.
