---
id: RT-agent-call-batch
area: RT
title: Fan out a bounded batch of agent calls
persona: Bruno
journey: J-delegate-work-to-an-agent
expected: A mixed batch returns HTTP 200 with independent accepted and typed rejected items, while an over-cap batch is rejected as one request.
entry_points: compozy__agent_call with {"tasks":[{"agent":"scout","prompt":"Map the package"},{"agent":"reviewer","prompt":"Review the diff","expect":{}}]}; HTTP and UDS POST /api/workspaces/{workspace_id}/calls with the same tasks payload; compozy call list --state queued,running,completed --limit 8; compozy config get calls.max_batch
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-golden-path; RT-delegation-depth-and-caps; RT-delegation-attention-signals
---

Submit two valid agents and one unknown agent, then submit a batch above the configured maximum.

An empty batch is its own case (`call_batch_empty`). Confirm the over-cap rejection leaves nothing
partially run, and that a fan-out reaches the caller's transcript as one fan-out card rather than one
card per task.
