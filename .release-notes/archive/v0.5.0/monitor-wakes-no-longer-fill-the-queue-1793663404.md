---
title: Monitor wakes no longer fill the session queue
type: fix
---

Monitors that fire during a long turn now merge into one queue entry that reads "N monitors fired" and carries every outcome in firing order. Before, each wake took its own row, and the composer could show `10 queued · full` while your own messages were still welcome. Only your messages count toward `full` and `queue_cap` now.

On a provider that accepts input mid-turn, the merged wake goes into the running turn after `session.messaging.queue_promote_after` (2 minutes by default), behind anything you already queued. Other providers get it at the next turn boundary, as one turn. `compozy__session_inputs_list` also no longer fails on these entries.

Limits:

- Past 64 KiB, a wake keeps its outcome and drops its output tail, with a note to read the terminal.
- Past 64 merged wakes, a new one waits and retries until the entry is delivered or its retry window ends.
- After a restart, a wake whose delivery was never confirmed goes back in the queue, so the agent can see the same outcome twice. This is deliberate: repeating an outcome is safe, and losing one is not.

Migration notes: no action is needed. Entries queued before the upgrade stay separate and deliver as before. `coalesced_wakes` is a new optional field on queue entries, and `input.pre_submit` hooks now also run for wakes delivered mid-turn.
