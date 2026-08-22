---
id: LP-web-strategy-progress
area: LP
title: Read fan-out strategy progress and partial completion
persona: Bruno
journey: J-complete-partial-loop
expected: The progress panel names the declared strategy and threshold, counts lanes canceled by the strategy apart from failures and never-materialized lanes apart from pending ones, shows the partial badge with coverage numbers whenever completion_state is partial, and reports aggregate counts instead of per-lane rows on a wide fan-out.
entry_points: /loop-runs/$runId progress panel; outcome card
qa_status: pass
bug_ids: ""
fix_status:
retest_status: pass
fix_commits: ""
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/task07-scenario-walks.md; .compozy/tasks/loop-task-legibility/evidence/visual/task_05/VC-21; .compozy/tasks/loop-task-legibility/evidence/visual/task_05/VC-29
last_report: docs/qa/reports/2026-08-21-loop-task-legibility.md
overlaps: ""
---

story: As a Loop operator, I can see how a fan-out settled without mistaking a strategy cancellation for a failure or a partial result for a complete one.

src: .compozy/tasks/graph-eng/task_08.md
