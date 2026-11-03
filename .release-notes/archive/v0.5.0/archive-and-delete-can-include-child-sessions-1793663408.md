---
title: Archive and delete can include child sessions
type: feature
---

Archiving or deleting a parent used to leave its child sessions behind, still running if they were live, without telling you they existed. Now a session with children asks first: archive offers a checked "Also archive N child sessions", and delete offers an unchecked "Also delete N child sessions". A session with no children works as before, and archive stays one click.

From the terminal, use `--with-children` on `compozy session archive` and `compozy session remove`. `compozy__session_archive` accepts `with_children`, and HTTP gains the same option plus a route that counts a session's descendants. Leaving the option out keeps today's behavior.

Limits: unarchive brings back subagents only, so restore other children one at a time from the Archive view or with `compozy session unarchive <id>`. Delete isn't atomic: if it fails partway, the sessions it didn't reach stay and the error names the child.
