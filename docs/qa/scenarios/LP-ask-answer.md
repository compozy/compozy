---
id: LP-ask-answer
area: LP
title: Answer a parked Loop ask through the CLI
persona: Bruno
journey: J-supervise-loop-request
expected: A published Loop parks at an ask, `compozy loop requests` lists its redacted preview, `compozy loop request` returns the full redacted context, and one schema-valid `compozy loop respond` resumes the run with the answer as node output.
entry_points: compozy loop requests; compozy loop request; compozy loop respond; HTTP and UDS Loop request routes
qa_status: untested
bug_ids: ""
fix_status: none
retest_status: pending
fix_commits: ""
evidence: ""
last_report: ""
overlaps: ""
---

story: As a Loop operator, I can find and answer one human request without exposing private execution payloads or resuming the node twice.

src: .compozy/tasks/graph-eng/task_03.md
