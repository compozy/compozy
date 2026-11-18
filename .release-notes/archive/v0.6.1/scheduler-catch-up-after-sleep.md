---
title: Scheduled jobs follow their catch-up policy after your computer sleeps
type: fix
---

When your computer sleeps through several runs of a recurring job, the job now follows its `catch_up_policy` when your computer wakes up. Before, a daemon that kept running through the sleep replayed every missed run regardless of policy. `coalesce` and `run_once_on_catchup` run once for the latest missed time, `replay` runs each missed time in order, and `skip_missed` checks the earliest missed run against the grace period: outside it, the backlog is recorded as one skipped run; within it, the latest missed time runs once. The job keeps its original schedule afterwards.

Migration notes: no action needed.
