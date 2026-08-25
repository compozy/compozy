---
id: RT-agent-call-cancel
area: RT
title: Cancel a running agent call idempotently
persona: Bruno
journey: J-agent-call-cancel
expected: Cancel fences activation, stops the managed child, settles canceled once, and a repeated cancel returns the same terminal state.
entry_points: compozy call cancel; POST calls cancel; compozy__call_cancel
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-golden-path
---

Cancel a long-running call, probe the child process, repeat the cancellation, and inspect the stored call.
