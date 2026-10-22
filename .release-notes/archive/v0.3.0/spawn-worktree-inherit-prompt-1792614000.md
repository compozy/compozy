---
title: Spawned workers get their own worktree, the parent's grant, and a first prompt
type: feature
---

`compozy spawn`, `compozy__session_spawn`, and `POST /api/agent/spawn` can now:

- bind the child to a worktree with `--worktree <ref>` or `--new-worktree[=name] --base --branch`.
  The checkout becomes the child's working directory and provider sandbox root.
- keep the parent's permission grant with `--inherit-permissions`, and select parent-granted tools
  by toolset with `--toolset`. Neither widens the parent.
- send the first prompt with `--prompt`. The parent's reply wake then carries the child's final
  answer.
- target another registered workspace with `--workspace` together with a worktree. The daemon
  authorizes the workspace before it resolves or creates the worktree there.

In a session, `compozy__workspace_list` now also lists the other workspaces the session's
permission mode lets it address. A refused spawn now names the field, value, and rule that
refused it, and a worktree created for it is removed again. An exhausted `max_children` budget
fails with `spawn_budget_exhausted` and the live count instead of a generic conflict; subagents no
longer count against it. The native session tools' output schemas now match the session runtime they
return, so `compozy__session_create` no longer reports a schema mismatch for a session it created.
