---
id: RT-agent-call-batch
area: RT
title: Fan out a bounded batch of agent calls
persona: Bruno
journey: J-agent-call-batch
expected: A mixed batch returns HTTP 200 with independent accepted and typed rejected items, while an over-cap batch is rejected as one request.
entry_points: compozy__agent_call tasks; POST /api/calls; compozy call list
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-golden-path
---

Submit two valid agents and one unknown agent, then submit a batch above the configured maximum.
