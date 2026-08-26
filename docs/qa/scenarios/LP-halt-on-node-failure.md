---
id: LP-halt-on-node-failure
area: LP
title: Stop a Loop on its first node failure
persona: Bruno
journey: J-configure-and-run-loop
expected: A Loop configured with reattempt_strategy halt ends failed on the first node failure without admitting another generation or reservation, and an explicit operator rerun remains available.
entry_points: compozy loop configure; compozy loop run; compozy loop status; compozy loop rerun; Web Loop Configure
qa_status: pass
bug_ids: BUG-20260826-halt-rerun-busy
fix_status: fixed
retest_status: pass
fix_commits: 151e299e6
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/logs/explicit-rerun-current-head.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/logs/terminal-status-current-head.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/logs/api-status-current-head.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/logs/web-terminal-current-head.json
last_report: docs/qa/reports/2026-08-26-loop-issues-fixes.md
overlaps: LP-time-travel-rerun; LP-web-loop-configure-modal
---

Use a deterministic failing node. Read status and node history after failure, wait past the ordinary
reattempt admission window, then start one explicit rerun and confirm it is operator-owned.

QA 2026-08-26 retest: generation 1 halted on `load_tasks`; the explicit operator rerun admitted
generation 2 across the graph's unmaterialized downstream nodes. The deterministic failure settled
again, and no automatic generation 3 appeared after the admission window.
