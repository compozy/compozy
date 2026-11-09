---
title: Automation runs stay out of the session list
type: fix
---

Sessions started by an automation job or trigger no longer show up in the session list, the board or their counts, and a new run no longer takes the selection. Before, a scheduled run appeared at the top of the list as "New session" with a working spinner while it ran. Each run stays reachable from the Automations run view and through search, and a run that needs you still reaches you through attention toasts and notifications.

To see them, turn on **Show automation runs** in the list's or board's **View options**. Each window remembers the choice. A run you prompted yourself stays on the list. Loop and Task sessions are unchanged. The dock's Sessions launcher now opens your newest own session, never an automation run.

Migration notes: no action needed; existing runs follow the same rule after upgrading. `compozy session list` and `compozy__session_list` keep returning every session. The API does too, unless a client passes the new `exclude_automation=true` query parameter on `GET /api/sessions` or `GET /api/sessions/facets`. `exclude_finished_automation=true` keeps its meaning.
