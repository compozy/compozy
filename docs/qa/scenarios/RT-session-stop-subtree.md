---
id: RT-session-stop-subtree
area: RT
title: Drain a governed session subtree before stopping its root
persona: Bruno
journey: J-session-stop-subtree
expected: Subtree stop fences new work, closes every open descendant call, stops each child once, and reports stopped children, closed calls, and preserved results.
entry_points: compozy session stop --subtree; session stop API; compozy__session_stop
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-cancel
---

Build a three-level call tree with one completed result, drain it, repeat the drain, and verify the report and processes.
