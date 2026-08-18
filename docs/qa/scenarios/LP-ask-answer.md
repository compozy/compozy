---
id: LP-ask-answer
area: LP
title: Answer a parked Loop ask through the CLI
persona: Bruno
journey: J-supervise-loop-request
expected: A published Loop parks at an ask, `compozy loop requests` lists its redacted preview, `compozy loop request` returns the full redacted context, and one schema-valid `compozy loop respond` resumes the run with the answer as node output.
entry_points: compozy loop requests; compozy loop request --item; compozy loop respond --item; GET /loop-requests over HTTP and UDS; GET /loop-runs/:id/nodes/:node/request over HTTP and UDS; POST /loop-runs/:id/nodes/:node/respond over HTTP and UDS; compozy__loop_requests; compozy__loop_request; compozy__loop_respond; /docs/loops/human-requests; /docs/loops/running
qa_status: pass
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: docs/qa/reports/2026-08-18-graph-eng.md
overlaps: ""
---

story: As a Loop operator, I can find and answer one human request without exposing private execution payloads or resuming the node twice.

src: .compozy/tasks/graph-eng/task_03.md
