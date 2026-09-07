---
title: Recover Loop runs without losing results or reporting an old failure as current
type: fix
---

Carried external results with identical descriptors are deduplicated, fixing task-result reads and daemon startup during recovery while retaining corruption checks for conflicting content. Current-generation activity and blockers no longer inherit an earlier generation's failure or quarantine.

Reruns respect immutable generation lineage and allocate fresh Goal binding epochs. Completed generation outputs reject stale nonterminal overwrites, and new rounds emit one generation-start event. Built-in command judges use the daemon-matched executable and environment.

The run page leads with outcomes and action results. It previews a few outputs, keeps Details available, folds quiet control and skipped steps, and shows terminal retry guidance only when the existing planner accepts that recovery.

PRs: [#547](https://github.com/compozy/compozy/pull/547), [#554](https://github.com/compozy/compozy/pull/554).
