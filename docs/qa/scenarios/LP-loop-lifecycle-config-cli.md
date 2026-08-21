---
id: LP-loop-lifecycle-config-cli
area: LP
title: Tune Loop lifecycle defaults through structured configuration
persona: Ada
journey: J-tune-loop-lifecycle-defaults
expected: The CLI accepts all documented delivery, watch, global breaker, and reconciliation lifecycle paths, fresh reads preserve independent values, and invalid input names the exact path without changing the last valid configuration; loops.reconcile_interval defaults to 1m, rejects a non-positive duration with "reconcile_interval must be positive" while preserving the prior value, is restart-required like every other loops key, and the daemon still runs one sweep at boot whatever the interval says.
entry_points: compozy config set <key> <value>; compozy config get <key>; compozy config set loops.reconcile_interval <duration>; compozy config get loops.reconcile_interval -o json; config.toml [loops]; /docs/cli/config/set
qa_status: untested
bug_ids: BUG-20260802-loop-lifecycle-config-unsupported
fix_status: fixed
retest_status: pass
fix_commits: Task 01 checkpoint
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-lifecycle-config-20260802-070909-601039-lab/qa-artifacts/qa/observed-results.md
last_report: docs/qa/reports/2026-08-02-loop-lifecycle-config.md
overlaps: LP-loop-config-file-snake-case; MS-workspace-resolution-chain; LP-terminal-loop-settlement
---

This scenario covers the 18 per-family lifecycle paths plus the two global breaker paths. The
autopause list is intentionally excluded from agent mutation and is validated through file-based
configuration instead.

2026-08-02 retest: all 20 lifecycle paths persisted distinct values through public CLI writes and
fresh reads. Out-of-range and malformed-duration writes preserved the prior values, while the
autopause list remained unavailable to agent mutation.

QA impact 2026-08-21: task_01 added `loops.reconcile_interval` — a 21st path in the same family and
the same behavior — so this row absorbs it rather than minting a sibling config scenario. The
surface changed, so the 2026-08-02 `pass` is reset to `untested`; the prior verdict's evidence and
report stay recorded until the next walk replaces them. The new path carries three facts the other
twenty do not: a positive-duration validation with a named message, a boot sweep that runs once
regardless of the configured interval, and the fact that only later sweeps observe a changed value.
