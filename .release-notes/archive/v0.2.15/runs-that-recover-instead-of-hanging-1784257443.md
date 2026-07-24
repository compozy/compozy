---
title: Runs that recover instead of hanging
type: highlight
---

Compozy now detects silent ACP stalls, retries once from a clean worktree, and parks persistent failures with journals and worktrees preserved for triage. Parallel parents can reap wedged children, terminal commands are cancellable and bounded, and the UI reports stalled, retrying, recovered, and parked outcomes. (PR #230)
