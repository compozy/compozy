---
id: LP-amend-rerun
area: LP
title: Amend a parked Loop output before a targeted rerun
persona: Bruno
journey: J-replay-loop-history
expected: A schema-valid amendment appends provenance without changing the recorded generation output, run detail exposes the bounded amendment, and a later resume or targeted rerun consumes the newest effective value.
entry_points: compozy loop node amend --item; compozy loop rerun --item; compozy loop status; POST /loop-runs/:id/nodes/:node/amend over HTTP and UDS; POST /loop-runs/:id/rerun over HTTP and UDS; GET /loop-runs/:id over HTTP and UDS; compozy__loop_node_amend; compozy__loop_rerun; compozy__loop_status; /docs/loops/running
qa_status: untested
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: ""
overlaps: ""
---

story: As a Loop operator, I can repair one parked output while preserving the immutable execution record and use that repair in subsequent work.

src: .compozy/tasks/graph-eng/task_04.md
