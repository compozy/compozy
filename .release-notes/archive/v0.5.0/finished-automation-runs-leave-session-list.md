---
title: Finished automation runs leave the session list
type: feature
---

Sessions started by an automation job or trigger now leave the session list, the board and their counts once they finish cleanly, so a weekly job no longer adds a permanent row. Nothing is archived or deleted: each run stays reachable from the Automations run view and through search, including a search in a grouped list.

A run stays on the list if it failed, if it or a child session is waiting on you or still working, or if you prompted it yourself. A failed run stays until you archive it. Opening a finished run doesn't bring it back. Loop and Task sessions are unchanged, including automation runs handed to a Loop or a Task.

Migration notes: existing runs follow the same rule after upgrading, with no action needed. `compozy session list` and `compozy__session_list` keep returning every session. The API does too, unless a client passes the new `exclude_finished_automation=true` query parameter.
