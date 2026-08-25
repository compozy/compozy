---
id: RT-agent-call-golden-path
area: RT
title: Complete a typed agent call from creation through result fetch
persona: Bruno
journey: J-agent-call-golden-path
expected: A typed call is accepted asynchronously, await reaches completed, and result returns the complete admitted JSON without losing profile or workspace ownership.
entry_points: compozy call; compozy call await; compozy call result; HTTP calls API
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Walk the create → await → result flow in both Global and workspace scope, then compare the CLI and HTTP projections.
