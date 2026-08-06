---
title: Loop runs you can debug from the run page
type: fix
---

A Loop run that failed used to be a dead end: every attempt died with "The agent output did not satisfy the action output schema", the node was quarantined, "Open quarantine entry" opened an empty sheet, "Open session" returned 404, the cell task sat in "Queued · attempt 1 of 10" forever, and Usage confidently reported `0 / ~$0.00`. The agent had actually answered correctly every time — the daemon joined streamed text fragments with a newline, which landed inside a JSON string and corrupted a valid reply. That joiner is fixed, and so is every surface that made the failure impossible to read. (#324)

- The agent now sees the authored `output_schema` in its prompt instead of prose that never said "JSON", extraction validates every candidate object newest-first (a quoted `package.json` no longer shadows the real answer), and the failure cause carries the underlying detail instead of one generic sentence.
- Quarantine is routed to the node that actually failed. Parked consumers collapse into a single row — "**execute task is quarantined** — collect, review, verify and approve are parked behind it until it is requeued" — with one button that opens the producer's entry, and `node_attention_flagged` is finally emitted when a run parks.
- Loop cells no longer stall in `ready` after a failed run: quarantine parks them as needs-attention and requeueing clears the park. The misleading `of 10` attempt ceiling is gone, since the Loop owns the retry budget.
- Daemon-claimed runs stop writing placeholder session ids, the real ACP session is bound to the lease under a claim token, and run detail exposes `generations[].outputs[].session_id` — so "Open session" works from the hero and from every node row that has one.
- The task list nests Loop cells under their coordinator with an escalation-first summary ("9 subtasks · 1 needs attention · 2 running") and readable identities like `g2.execute_task` instead of `loop.lo`.
- When a provider reports no tokens, Usage now reads "not reported" and "—" instead of a confident zero; the cost estimate returns only when tokens exist.

Migration notes: adds the `attention_producer_node_id` column to `loop_node_controls` through migration `00055`; run-detail payloads gain `node_controls[].attention_producer_node_id` and `generations[].outputs[].session_id`.
