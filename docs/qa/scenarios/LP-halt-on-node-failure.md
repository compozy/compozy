---
id: LP-halt-on-node-failure
area: LP
title: Stop a Loop on its first node failure
persona: Bruno
journey: J-configure-and-run-loop
expected: A Loop configured with reattempt_strategy halt ends failed on the first node failure without admitting another generation or reservation, and an explicit operator rerun remains available.
entry_points: compozy loop configure; compozy loop run; compozy loop status; compozy loop rerun; Web Loop Configure
qa_status: fail
bug_ids: BUG-20260826-halt-rerun-busy
fix_status: pending
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-20260826-192350-462800-lab/qa-artifacts/qa/logs/explicit-rerun.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-20260826-192350-462800-lab/qa-artifacts/qa/logs/halt-run-status-after-wait.json
last_report: docs/qa/reports/2026-08-26-loop-issues-fixes.md
overlaps: LP-time-travel-rerun; LP-web-loop-configure-modal
---

Use a deterministic failing node. Read status and node history after failure, wait past the ordinary
reattempt admission window, then start one explicit rerun and confirm it is operator-owned.

QA 2026-08-26: automatic halt passed, but explicit rerun returned `rerun_busy` because an untouched
downstream node remained pending. Filed BUG-20260826-halt-rerun-busy.
