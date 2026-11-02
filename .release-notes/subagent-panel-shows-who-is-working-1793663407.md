---
title: The subagent panel shows who is working, on what, and with which model
type: feature
---

Each row in the composer's subagent panel now shows the title, model and kind, the current action and the elapsed time, so a subagent no longer reads "general" and "Not reported". Click a row to slide the panel to that subagent: status, time, tools, tokens, what it was asked to do, its result and its tool activity. Back and Escape return to the row. A bar in the header shows one segment per subagent, filling as they finish, fail, work and queue.

Limits: the provider reports a model alias, so a row reads `Sonnet`, not its full version. Token and tool counts exist only for subagents the provider runs itself. Nothing reports a percent or a per-subagent cost, so neither is shown.

Migration notes: subagent payloads and `compozy__subagent_status` gain an optional `usage` field. On provider-run subagents, `role` now holds the subagent type instead of the fixed value `general`, so update any script that matches on it.
