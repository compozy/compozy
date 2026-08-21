---
id: LP-terminal-loop-settlement
area: LP
title: Terminal Loop runs leave no live execution records
persona: Ada
journey: J-loop-terminal-recovery
expected: Natural completion, cancellation, kill, crash recovery, and retention-orphan repair leave no claimable task runs; task timelines expose loop_run_terminal, reconciled_run_terminal, or run_missing as structured reasons.
entry_points: compozy daemon; compozy loop status; compozy task timeline; compozy config get loops.reconcile_interval -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Seed terminal and missing-run ownership shapes before daemon start. Verify the boot barrier removes
claim eligibility before recovery, a second boot emits no duplicate repair event, active Loop runs
remain unchanged, and the configured interval affects only later sweeps.
