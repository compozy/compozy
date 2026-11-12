---
title: Scheduled jobs follow their catch-up policy after your computer sleeps
type: fix
---

When your computer sleeps through several runs of a recurring job, the job now follows its `catch_up_policy` once the daemon is running again. Before, it replayed every missed run regardless of policy. `coalesce` and `run_once_on_catchup` run once for the latest missed time, `replay` runs each missed time in order, and `skip_missed` records the missed runs as skipped, or runs the latest one if it is still within the configured grace period. The job keeps its original schedule afterwards.

Migration notes: no action needed.
