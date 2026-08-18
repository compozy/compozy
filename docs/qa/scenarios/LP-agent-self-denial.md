---
id: LP-agent-self-denial
area: LP
title: Deny an agent from operating its own Loop run
persona: Ada
journey: J-supervise-loop-request
expected: An agent with loops.respond or loops.timetravel can operate an unrelated permitted run, while the run starter and every child in its durable spawn chain receive respond_self_denied or timetravel_self_denied; missing capabilities, cross-workspace targets, and stale lineage fail closed.
entry_points: compozy__loop_requests; compozy__loop_request; compozy__loop_respond; compozy__loop_rerun; compozy__loop_fork; POST /loop-runs/:id/nodes/:node/respond over HTTP and UDS; POST /loop-runs/:id/rerun over HTTP and UDS; POST /loop-runs/:id/fork over HTTP and UDS; skills/compozy/references/loops.md
qa_status: pass
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: ""
last_report: docs/qa/reports/2026-08-18-graph-eng.md
overlaps: LP-ask-answer; LP-time-travel-rerun; LP-time-travel-fork
---

story: As an agent operator, I can delegate answers without allowing the agent that started a run, or its spawned descendants, to approve their own work.

src: .compozy/tasks/graph-eng/task_03.md
