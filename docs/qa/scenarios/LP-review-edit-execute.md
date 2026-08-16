---
id: LP-review-edit-execute
area: LP
title: Edit a reviewed Loop action before execution
persona: Bruno
journey: J-supervise-loop-request
expected: A reviewed action opens before task creation, a schema-valid edit admits the exact replacement parameters, the action runs once, and sibling fan-out cells remain unchanged.
entry_points: compozy loop requests; compozy loop request; compozy loop respond --decision edit; HTTP and UDS Loop request routes; compozy__loop_respond
qa_status: untested
bug_ids: ""
fix_status: none
retest_status: pending
fix_commits: ""
evidence: ""
last_report: ""
overlaps: ""
---

story: As a Loop operator, I can correct a reviewed action's resolved parameters before the action executes exactly once.

src: .compozy/tasks/graph-eng/task_04.md
