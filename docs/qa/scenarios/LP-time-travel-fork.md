---
id: LP-time-travel-fork
area: LP
title: Fork a linked Loop run from history
persona: Bruno
journey: J-replay-loop-history
expected: Fork creates a child with a settled fork_seed generation, validates input overrides, runs the full body in generation 2, exposes two-way lineage, and leaves the source byte-identical.
entry_points: compozy loop fork; POST /loop-runs/:id/fork over HTTP and UDS; compozy__loop_fork gated by loops.timetravel; web Fork dialog and lineage; /docs/loops/time-travel; /docs/loops/running
qa_status: untested
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: ""
overlaps: ""
---

story: As a Loop operator, I can start a new attempt from a historical baseline without mutating its source.

src: .compozy/tasks/graph-eng/task_07.md
