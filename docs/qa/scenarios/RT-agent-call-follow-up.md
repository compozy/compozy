---
id: RT-agent-call-follow-up
area: RT
title: Revive a parked call child for follow-up work
persona: Bruno
journey: J-delegate-work-to-an-agent
expected: Calling a parked child session revives the same child, preserves its prior context, and produces a new durable call result.
entry_points: compozy call ses_01JBD8G2MZTX "Check the tests too"; compozy call await call_01JBD8H9PW2M --timeout 120s; compozy session status ses_01JBD8G2MZTX; HTTP and UDS POST /api/workspaces/{workspace_id}/calls with {"target":{"session_id":"ses_01JBD8G2MZTX"},"prompt":"Check the tests too"}; compozy__agent_call with {"session_id":"ses_01JBD8G2MZTX","prompt":"Check the tests too"}
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-golden-path; RT-parked-child-idle-ttl
---

Complete an initial call, wait for the child to park, then call the child session id and verify continuity.

Because a follow-up reuses its child, `child_session_id` is not unique across calls — confirm the
second call gets its own call id while pointing at the same child, and that the delegation tree still
renders one subtree rather than two.
