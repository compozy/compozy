---
title: Ask the agent when nothing matches
type: feature
---

A query with no strong result no longer dead-ends. The palette offers one visually distinct `Ask agent: '<query>'` row; pressing Enter creates a session with the workspace's default agent and uses the query as the opening prompt. (#441)

- Nothing is sent before Enter. Typing never carries the query to a provider, and a rapid double Enter still creates exactly one session.
- A weak-but-real match keeps both the results and the fallback row; only a query below the served threshold is fallback-only.
- With no workspace default agent, Enter opens the agent picker first. If the session fails to start, the failure names its reason and the palette reopens with your query intact.
- Turn it off in Settings → Palette, or set `fallback_targets = []`. Both report the same desired state, and the row disappears immediately.

```toml
[cmd_palette]
# The current runtime accepts "agent". Use [] to disable the fallback row.
fallback_targets = ["agent"]
```
