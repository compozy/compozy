---
id: LP-time-travel-rerun
area: LP
title: Rerun a terminal Loop from one settled node
persona: Bruno
journey: J-replay-loop-history
expected: Rerun opens one operator_rerun generation, reruns the selected lane and transitive dependents, carries unrelated cells, preserves provenance, and safely replays an explicit request id.
entry_points: compozy loop rerun --item; POST /loop-runs/:id/rerun over HTTP and UDS; compozy__loop_rerun gated by loops.timetravel; web Rerun dialog; /docs/loops/time-travel; /docs/loops/running
qa_status: untested
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: ""
overlaps: LP-amend-rerun
---

story: As a Loop operator, I can retry downstream work without rebuilding unaffected history.

src: .compozy/tasks/graph-eng/task_07.md
