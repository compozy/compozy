---
title: Native tool supervision follow-ups
type: fix
---

Native tools and subagent supervision handle several edge cases more reliably. One failed native-result row no longer stops other replay work. A failed MCP discovery attempt is retried by peers instead of being shared as a complete tool list, while healthy tools stay available. Monitors on child sessions no longer depend on whether you viewed the child. Scheduler and harness heartbeat wakes use the agent's current heartbeat policy. A native approval that expires is reported as `approval_timed_out` with guidance to ask again, instead of looking like an explicit rejection.

Migration notes: no action needed.
