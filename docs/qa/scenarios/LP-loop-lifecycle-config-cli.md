---
id: LP-loop-lifecycle-config-cli
area: LP
title: Tune Loop lifecycle defaults through structured configuration
persona: Ada
journey: J-tune-loop-lifecycle-defaults
expected: The CLI accepts all documented delivery, watch, and global breaker lifecycle paths, fresh reads preserve independent values, and invalid input names the exact path without changing the last valid configuration.
entry_points: compozy config set; compozy config get
qa_status: pass
bug_ids: BUG-20260802-loop-lifecycle-config-unsupported
fix_status: fixed
retest_status: pass
fix_commits: Task 01 checkpoint
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-lifecycle-config-20260802-070909-601039-lab/qa-artifacts/qa/observed-results.md
last_report: docs/qa/reports/2026-08-02-loop-lifecycle-config.md
overlaps: LP-loop-config-file-snake-case; MS-workspace-resolution-chain
---

This scenario covers the 18 per-family lifecycle paths plus the two global breaker paths. The
autopause list is intentionally excluded from agent mutation and is validated through file-based
configuration instead.

2026-08-02 retest: all 20 lifecycle paths persisted distinct values through public CLI writes and
fresh reads. Out-of-range and malformed-duration writes preserved the prior values, while the
autopause list remained unavailable to agent mutation.
