---
id: LP-request-expiry
area: LP
title: Expire a parked Loop request exactly once
persona: Bruno
journey: J-supervise-loop-request
expected: An unanswered ask follows its authored expiry or loops.defaults.<kind>.requests.expire_after, emits each escalation once, closes as expired after a daemon restart, takes the declared route once, rejects a late answer with request_expired, and preserves the config value through get, set, unset, show, and reload.
entry_points: loop ask runtime; scheduler due scan; daemon restart; compozy loop request; compozy loop respond; compozy config get|set|unset|show; config.toml loops.defaults.delivery.requests.expire_after and loops.defaults.watch.requests.expire_after; /docs/loops/human-requests; site config reference
qa_status: pass
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: docs/qa/reports/2026-08-18-graph-eng.md
overlaps: LP-ask-answer
---

story: As a Loop operator, I can trust an unanswered request to reach one deterministic expiry outcome even when the daemon restarts near the deadline.

src: .compozy/tasks/graph-eng/task_03.md
