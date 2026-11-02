---
title: Stays-on agents run at their configured permission level
type: fix
---

Stays-on agents now run at the permission level set on them, or at `[permissions].mode` when they have none. This applies whether you or an agent created the instance. Before this release an agent set to `approve-all` ran at `approve-reads`, so unattended work stopped at its first write with nothing explaining why.

Sessions started by a stays-on agent no longer take its level. Each uses its own agent's `permissions`, then `[permissions].mode`. A `permission_mode` above that level is refused. Tool, skill, MCP server, and path limits still apply, and triage stays read-only.

**After upgrading:** a stays-on agent moves to its configured level the next time its main session starts. Any stays-on agent that resolves to `approve-all`, including through the default `[permissions].mode`, can now act without a prompt. To keep one asking first, set `permissions: approve-reads` on it.
