---
id: RT-agent-call-follow-up
area: RT
title: Revive a parked call child for follow-up work
persona: Bruno
journey: J-agent-call-follow-up
expected: Calling a parked child session revives the same child, preserves its prior context, and produces a new durable call result.
entry_points: compozy call with session id; call await; session status
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-golden-path
---

Complete an initial call, wait for the child to park, then call the child session id and verify continuity.
