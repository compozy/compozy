---
id: TA-task-result-contract
area: TA
title: Complete a task against its run-start result contract
persona: Bruno
journey: J-contract-a-task-result
expected: Task authoring accepts expect and budget fields, reads echo the digest and effective budget, and completion validates the immutable run-start snapshot with one resubmission.
entry_points: compozy task create "Review the patch" --expect @review-contract.json and compozy task update task_01JBD9AAAA --expect @review-contract-v2.json; HTTP and UDS POST /api/tasks and PATCH /api/tasks/{id} with {"expect":{},"result_budget":"256KiB","result_overflow":"store"}; HTTP and UDS POST /api/agent/tasks/{run_id}/complete with {"claim_token":"…","result":{"verdict":"approved"}}; task reads that return expect_digest and the effective budget; compozy__task_create and compozy__task_update with the same contract and budget fields
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: TA-task-result-default-budget; RT-call-return-contract-repair; LP-loop-contract-regime-adoption
---

Start a contracted run, update the task contract mid-run, submit one invalid result, then resubmit against the original snapshot.

Both accepted contract forms belong in the walk: a full JSON Schema and the example-shape shorthand
must normalize to the same canonical contract and pin the same digest, and anything that is neither
must fail with `call_expect_invalid` carrying the schema error verbatim. The invariant this row
exists for is that a run is judged against the contract it started with — so the mid-run update must
apply to future runs only, and a retry after a crash must be re-scored against the same durable
start-time snapshot.
