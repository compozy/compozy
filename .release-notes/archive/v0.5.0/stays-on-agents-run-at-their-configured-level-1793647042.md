---
title: Stays-on agents run at their configured permission level
type: feature
---

A stays-on agent runs at the `permissions` level on its own definition, whether you or an agent created the instance.

- **No level set:** it stays at `approve-reads`, or at `[permissions].mode` when that is narrower. The bundled `orchestrator` sets no level, so it stops for your approval when it writes.
- **`approve-all`:** it acts without prompts.
- **Sessions it starts:** they don't take its level. Each uses its own agent's `permissions`, then `[permissions].mode`, so a stays-on agent at `approve-reads` can start a session that runs at `approve-all`. A `permission_mode` above that session's level is refused.

Tool, skill, MCP server and path limits still apply, and check-ins stay read-only. The level is read each time a session starts, so a session that can write the agent's `AGENT.md` or the project's `.compozy/config.toml` can change it. Treat write access to those files as authority over the level.

### Migration

Stays-on agents are new in this release, so upgrading from v0.4.8 changes nothing here. If you already run them on a pre-release build, each one moves to its configured level the next time its main session starts. A stays-on agent with no level stays at `approve-reads`. One set to `approve-all` used to be held at `approve-reads` and acts without prompts after upgrading.

To keep one asking first, set `permissions: approve-reads` on it or remove the level. This covers only the agent's own actions, so set a level on the agents it starts as well.
