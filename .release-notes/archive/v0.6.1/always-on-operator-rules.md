---
title: Always-on agents follow operator rules and know their peers
type: feat
---

An always-on agent's memory now separates **rules** from **notes**. A rule is an instruction you write for the agent; it is delivered with every turn, survives provider compaction, and takes precedence over notes. Add one with `compozy agent memory add <agent> "<text>" --kind rule` or from the agent's Memory editor. The agent's own memory tools write notes only and cannot change a rule; a `compozy agent memory` or HTTP call made with an agent session's identity is refused the same way (403 `agent_memory_rule_protected`). This guards the agent's tools; it is not a sandbox against an agent that edits files or drops its identity.

Always-on agents also receive a roster of the other always-on agents they can reach, with each one's role and the input forms it accepts, so work goes to the agent that owns it. A definition declares its forms with an `## Input` block or the new `accepts` key.

On Claude, an always-on agent no longer inherits the MCP servers and claude.ai connectors of your own Claude setup, which filled its context with hundreds of unrelated tool names outside CompozyOS tool policy.

Migration notes: existing memory files need no change; every existing line stays a note. If an always-on Claude agent relied on an MCP server from your own Claude configuration, declare it in the agent's `mcp_servers` or set `lean_environment: false` in its `AGENT.md`.
