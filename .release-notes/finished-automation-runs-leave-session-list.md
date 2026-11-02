---
title: Finished automation runs leave the session list
type: feat
---

Sessions started by an automation job or trigger now leave the session list and the board once they finish cleanly, so a weekly job no longer adds a permanent row. Nothing is archived or deleted: each run stays reachable from the Automations run view and through search.

A run stays on the list if it failed, if it or a child session is waiting on you or still working, or if you prompted it yourself. A failed run stays until you archive it. Loop and Task sessions are unchanged, and so are `compozy session list` and `compozy__session_list`.

Existing runs follow the same rule after upgrading, with no action needed.
