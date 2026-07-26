---
title: Task blocking, recovery, and needs-attention triage
type: feature
---

Tasks are now first-class to block, triage, and recover. A task can be explicitly blocked and unblocked with `agh task block <id>` and `agh task unblock <id>`, and every block is kept as durable history you can inspect with `agh task blocks <id>`. A new `needs_attention` status surfaces tasks that stalled and need a human or coordinator to step in — task details now carry the blocked reason and the wake creator so it is clear why a task is waiting and who nudged it. Clear the state with `agh task recover <id>`, recover a specific stalled run with `agh task run recover <run-id>`, or let an agent do it through the `agh__task_recover` native tool and the HTTP/UDS API. Task completion now returns the IDs of any tasks it created, and stronger validation rejects invalid created-task references before they are persisted. Claim tokens are redacted across task outputs and hook payloads, and the new `needs_attention_after` setting controls how long a task may wait before it is flagged for attention.
