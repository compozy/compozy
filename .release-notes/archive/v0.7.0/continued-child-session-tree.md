---
title: Continued child sessions stay under their parent in the session list
type: fix
---

Continuing a child session no longer moves the new session to the top of the session list. When an orchestrator continues a worker, for example after a daemon restart stopped it, the continuation now appears in the orchestrator's thread next to the session it continues. This also holds when a session is continued more than once. Continuations of top-level sessions and forks still show as top-level sessions, and the continued session still names its source as "Continued from".

Only placement in the list changes. Permissions, lifecycle and child counts work as before: a continuation is not counted as one of the parent's children and does not stop when the parent stops. Session list and detail reads now include `tree_parent_session_id`, the parent a continuation is listed under. Sessions continued before this release move under their parent the next time the list loads; no migration is needed.
