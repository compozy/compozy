---
title: App status asks the app instead of guessing from the process
type: fix
---

`compozy app status` and `compozy app open` decided whether the desktop app was running by matching a recorded process ID and its start timestamp. When that timestamp comparison did not line up, a perfectly healthy app was reported as not running, and `app open` refused to reuse it. Both commands now probe the app's own control socket and take its answer. (#494)

- A control channel that reports not running, or is unavailable, still resolves to "not running" instead of failing the command.
- Any other probe failure is surfaced as an error rather than silently read as a stopped app.
