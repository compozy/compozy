---
title: Preserve session authority and finish accepted cleanup and layout changes
type: fix
---

Legacy sessions belonging to delegated always-on instances again retain their own tool and MCP
server restrictions after upgrading. Corrected catalog permission intent also applies when a
development build wrote stale intent markers into a session's metadata.

Deleting stopped history whose metadata is missing now removes its event database and attachments,
with the same owner checks and recoverable staging as normal deletion. Subagent cancellation finishes
its accepted settlement after a client disconnects. Failed roster removal leaves a terminal row that
can be deleted again even after its removal deadline expires. Archived cleanup bounds individual
deletions, including lifecycle-lock admission, and stops cached child
activity atomically. Raw session streams end on a replayed stop and fall back to polling after a failed
gap read.

Accepted layout replacements, migration actions, layout profile writes and client changes now survive
client disconnection within a bounded lifetime. Workspace checks and revision/version conflicts still
apply. Automation detail stays in its loading state while its window acquires live data, subagent
pages avoid duplicate rows, and removing an inspected subagent elsewhere closes its drawer after
confirmation for the current selection.
`compozy daemon stop` recognizes recycled process IDs. Automation waits and batch worktree removal
honor the configured client request budget.

### Migration

Migration 00204 appends a permission-intent correction; shipped 00202 and 00203 remain unchanged.
Stored lists and ancestor ceilings are retained. The original second-resolution 00202 cutoff is
preserved: rows in its ambiguous fractional second are not widened automatically. An affected user
can explicitly re-grant tool or MCP access to that session. Malformed historical permission JSON still
requires diagnosis rather than silently skipping shipped integrity failures.
