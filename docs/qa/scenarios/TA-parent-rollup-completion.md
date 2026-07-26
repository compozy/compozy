---
id: TA-parent-rollup-completion
area: TA
title: Complete a parent when every child completes
persona: Bruno
journey: J-complete-task-tree
expected: Completing the final child transitions the parent task to completed exactly once, while completing an earlier child leaves the parent non-terminal; the rollup is visible after refresh through Web and structured surfaces.
entry_points: web /tasks; task detail modal; CLI agh task list
qa_status: untested
bug_ids: BUG-20260713-parent-task-rollup-missing
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-consumer-saas-growth-20260714-194637-422214-lab/qa-artifacts/qa/task-rollup/; /Users/pedronauck/dev/qa-labs/agh-consumer-saas-growth-20260714-194637-422214-lab/qa-artifacts/qa/screenshots/task-parent-rollup-completed.png
last_report: docs/qa/reports/2026-07-14-consumer-saas-growth.md
overlaps: LP-042;TA-012
---

Linear issue AGH-71 is the named regression target.

2026-07-13: Owner clearing, recovery, task-role activation, and exactly-once child completion are fixed and passed. An earlier child correctly left the parent non-terminal, but after the final two real Cursor completions all three children are Completed while the parent remains Ready / Needs Attention. This is the direct AGH-71 failure.

2026-07-14 retest: fresh parent `task-a2b46ce593b5e75b` had no bound session and stayed nonterminal after child A. Child B's one real completion atomically settled the existing parent run and Task. Reload plus the Children tab showed one Completed parent run and both children Completed. AGH-71 passes; the separately tracked matching-Loop wake remains pending.

QA impact 2026-07-14: post-commit settlement publication now uses a bounded detached context and attempts parent completion effects after reconciliation errors. Reset pending a final-worktree replay.

2026-07-14 final-worktree control: the parent remained nonterminal before child C completed, then fresh reads showed one completed parent transition after the final child settlement. Retest promoted to pass.

2026-07-21: qa_status reset to untested — the opendesign redesigns restructured this scenario's web entry surface (task detail/run detail 3-tab IA, settings takeover shell, or providers page); the pass verdict predates that surface.
