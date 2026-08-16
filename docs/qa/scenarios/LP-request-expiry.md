---
id: LP-request-expiry
area: LP
title: Expire a parked Loop request exactly once
persona: Bruno
journey: J-supervise-loop-request
expected: An unanswered ask follows its authored or configured expiry, emits each escalation once, closes as expired after a daemon restart, takes the declared route once, and rejects a late answer with `request_expired`.
entry_points: loop ask runtime; scheduler due scan; daemon restart; compozy loop request; compozy loop respond
qa_status: untested
bug_ids: ""
fix_status: none
retest_status: pending
fix_commits: ""
evidence: ""
last_report: ""
overlaps: LP-ask-answer
---

story: As a Loop operator, I can trust an unanswered request to reach one deterministic expiry outcome even when the daemon restarts near the deadline.

src: .compozy/tasks/graph-eng/task_03.md
