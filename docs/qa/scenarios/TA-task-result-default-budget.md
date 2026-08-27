---
id: TA-task-result-default-budget
area: TA
title: Retain an uncontracted task result above 64 KiB
persona: Ada
journey: J-contract-a-task-result
expected: A task result larger than 64 KiB and no larger than the configured 256 KiB default budget completes successfully, retains the exact result bytes, and remains readable after daemon restart.
entry_points: compozy config get calls.results.default_budget, compozy config get calls.results.max_budget, and compozy config get calls.results.overflow; HTTP and UDS POST /api/tasks then POST /api/tasks/{id}/runs and POST /api/agent/tasks/{run_id}/complete; HTTP and UDS GET /api/task-runs/{run_id}; daemon restart
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: TA-task-result-contract; LP-loop-contract-regime-adoption
---

Flagged by the agent communications legacy-contract adoption. Exercise both sides of the old
64 KiB boundary and the configured default-budget boundary, including exact-byte verification
after restart. The agent communications QA phase owns execution and evidence.

Before constructing the result, read `calls.results.default_budget`,
`calls.results.max_budget`, and `calls.results.overflow` from the fresh QA home. Require the default
`256KiB` / maximum `4MiB` / `store` context or record the actual configured values, then derive the
payload boundary in exact bytes from the observed default. The passing payload must be larger than
64 KiB and no larger than that derived byte count; do not hard-code 256 KiB unless the fresh default
read proves it.

src: .compozy/tasks/agent-comms/task_02.md
