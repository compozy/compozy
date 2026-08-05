---
title: Complete Loop node lifecycle
type: feature
---

Loops now have a full declarative failure contract at the node level and precise repair controls at the operator level. Authors classify failures, declare retries with backoff, route errors, absorb them with `allow_fail`, set attempt timeouts and deadlines, emit `on_*` effects, and add durable wait nodes. Operators pause, resume, cancel, kill, or requeue individual nodes and list what is waiting, quarantined, retrying, or asking for attention — all from the CLI, HTTP, UDS, native tools, and MCP, without opening the web UI. (#305)

- `compozy loop cancel` drains a run safely and `compozy loop kill` closes it immediately; `compozy loop node pause|resume|cancel|kill|requeue` repairs a single node, and `compozy loop nodes --state waiting|quarantined|attention|retrying` inventories a run.
- Agents get the same controls through `compozy__loop_cancel`, `compozy__loop_kill`, `compozy__loop_node_pause`, `compozy__loop_node_resume`, `compozy__loop_node_cancel`, `compozy__loop_node_kill`, `compozy__loop_node_requeue`, and `compozy__loop_nodes`; `compozy__loop_status` now reports node lifecycle state.
- Repeated failure on one node quarantines that node instead of terminating the run, so independent lanes keep working, and the failure cause is classified rather than matched from a magic string.
- Long-running Loop-bound sessions are no longer killed on elapsed time. Liveness is judged from real evidence, and prolonged silence raises attention instead of ending the work.
- Defaults are tunable through `loops.defaults.delivery.*`, `loops.defaults.watch.*`, and `loops.breaker.*`, and new blocking lint rules reject invalid routes, impossible timing, malformed effects and waits, and watch sources without a stable identity.

Migration notes: `compozy loop stop` is deleted — the CLI verb, the HTTP route, and the `compozy__loop_stop` native tool. Choose `cancel` or `kill` explicitly. Extension watch sources must now declare `event_key`; a source without a stable event identity is rejected before a run starts.
