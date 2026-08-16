---
id: LP-agent-self-denial
area: LP
title: Deny an agent from answering its own Loop run
persona: Ana
journey: J-agent-manage-loop-request
expected: A capability-gated agent can answer an opted-in request from an unrelated run, while the run starter and every child in its durable spawn chain receive `respond_self_denied`; cross-workspace or stale lineage fails closed.
entry_points: compozy__loop_requests; compozy__loop_request; compozy__loop_respond; HTTP and UDS respond route
qa_status: untested
bug_ids: ""
fix_status: none
retest_status: pending
fix_commits: ""
evidence: ""
last_report: ""
overlaps: LP-ask-answer
---

story: As an agent operator, I can delegate answers without allowing the agent that started a run, or its spawned descendants, to approve their own work.

src: .compozy/tasks/graph-eng/task_03.md
