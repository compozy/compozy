---
title: Always-on agents run unattended at Accept edits and Auto-review
type: fix
---

An always-on agent at **Accept edits** (`approve-edits`) or **Auto-review** (`auto`) used to ask you before almost every CompozyOS tool call, including monitors, child sessions, worktrees, status reads and skill reads. The built-in Orchestrator couldn't run unattended unless you copied it at **Full access**.

- At those levels, CompozyOS tools that change things without destroying them now run without asking. Examples are creating sessions and worktrees, binding a worktree, renaming and linking sessions, task bookkeeping and layouts.
- These still ask: destructive and open-world tools, terminal commands, Loop and Automation tools, config, tool approvals, hooks, extensions, provider model changes, and creating, pausing or resuming agents.
- A call you approve at the provider prompt is no longer asked about a second time.
- CompozyOS tools now follow the session's own permission level, the same level the provider uses.
- The built-in Orchestrator now ships with `permissions: auto`.

`compozy__session_create` runs without asking at these levels and starts the child at the target agent's own level, which can be higher than the caller's.

Other always-on agent fixes:

- The **Model** row on an agent's home is now the model picker itself. A change saves at once, and **Use default** clears the agent's own choice.
- **Add always-on agent** is in the session list toolbar `+` in every view, not only in a project row.
- Changing an agent's avatar or color from its profile header saves again instead of failing with `invalid agent definition appearance`. An unsupported saved appearance no longer fails an agent read or `compozy agent duplicate`; the copy gets the default face.
- Cancelling your own monitor with `compozy__monitor_cancel` no longer reports "A wake-up didn't arrive".
