---
id: LP-review-edit-execute
area: LP
title: Edit a reviewed Loop action before execution
persona: Bruno
journey: J-supervise-loop-request
expected: A reviewed action opens before task creation, a schema-valid edit admits the exact replacement parameters, the action runs once, and sibling fan-out cells remain unchanged.
entry_points: compozy loop requests; compozy loop request; compozy loop respond --decision approve|edit|reject|respond; GET /loop-requests over HTTP and UDS; GET /loop-runs/:id/nodes/:node/request over HTTP and UDS; POST /loop-runs/:id/nodes/:node/respond over HTTP and UDS; compozy__loop_respond; /docs/loops/human-requests; /docs/loops/running
qa_status: pass
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: docs/qa/reports/2026-08-18-graph-eng.md
overlaps: ""
---

story: As a Loop operator, I can correct a reviewed action's resolved parameters before the action executes exactly once.

src: .compozy/tasks/graph-eng/task_04.md
