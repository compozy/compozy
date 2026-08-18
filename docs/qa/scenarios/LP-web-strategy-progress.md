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
retest_status:
fix_commits: ""
evidence: ""
last_report: docs/qa/reports/2026-08-18-graph-eng.md
overlaps: ""
---

story: As a Loop operator, I can see how a fan-out settled without mistaking a strategy cancellation for a failure or a partial result for a complete one.

src: .compozy/tasks/graph-eng/task_08.md
