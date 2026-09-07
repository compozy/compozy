---
title: Cancel now stops a Loop run for real, and public Kill operations are gone
type: breaking
---

Loop cancellation commits the terminal state before cleanup and stops every session that run owns, so a canceled run leaves nothing alive. Because cancel is now forceful, the separate Kill operation is removed from every public surface. (#509)

- Removed CLI commands: `compozy loop kill` and `compozy loop node kill`.
- Removed HTTP and UDS routes: `POST /workspaces/{workspace_id}/loop-runs/{run_id}/kill` and `POST /workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/kill`.
- Removed native tools: `compozy__loop_kill` and `compozy__loop_node_kill`.
- Public run transition and event vocabulary now exposes only cancellation. Stored `node_killed` events are projected as canceled.
- Authored Loop DSL now accepts `on_parent_close: terminate` or `abandon`. Stored `cancel` values are normalized to `terminate`.
- `compozy__loop_cancel` and `compozy__loop_node_cancel` — and their CLI, HTTP, UDS, and Web equivalents — now terminalize immediately instead of requesting cooperative cancellation.
- Sessions the run only borrowed are left running, workspace isolation is preserved, and a cleanup that fails is retried durably.

Callers still using a Kill route, command, or tool must move to the matching cancel surface; there is no public alias.
