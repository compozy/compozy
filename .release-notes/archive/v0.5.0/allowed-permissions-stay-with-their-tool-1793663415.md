---
title: Allowed permissions stay with their tool in the timeline
type: fix
---

An allowed permission no longer splits a tool group into `Used 2 tools / Allowed… / Used 2 tools`. It now sits under its tool, visible when you expand the group. Rejected, expired, pending and unmatched permissions keep their own row, and tool counts don't change.

Approvals of the native `compozy__terminal_exec` tool still show on their own row, because nothing links them to their tool call yet. In sessions that mostly run terminal commands, most allowed permissions will look the same as before.
