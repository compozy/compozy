---
id: LP-amend-rerun
area: LP
title: Amend a parked Loop output before a targeted rerun
persona: Bruno
journey: J-repair-loop-run
expected: A schema-valid amendment appends provenance without changing the recorded generation output, run detail exposes the bounded amendment, and a later resume or targeted rerun consumes the newest effective value.
entry_points: compozy loop node amend; compozy loop status; HTTP and UDS Loop node amend and run detail routes; compozy__loop_node_amend; compozy__loop_status
qa_status: untested
bug_ids: ""
fix_status: none
retest_status: pending
fix_commits: ""
evidence: ""
last_report: ""
overlaps: ""
---

story: As a Loop operator, I can repair one parked output while preserving the immutable execution record and use that repair in subsequent work.

src: .compozy/tasks/graph-eng/task_04.md
