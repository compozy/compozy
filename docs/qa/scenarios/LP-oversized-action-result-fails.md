---
id: LP-oversized-action-result-fails
area: LP
title: Bound oversized Loop action results without leaking a lease
persona: Bruno
journey: J-complete-partial-loop
expected: A Loop action result above 64 KiB but within the effective action budget succeeds through a durable result reference, while a result above that budget fails the owning node with a bounded diagnostic and releases its lease across fresh CLI, HTTP, UDS, Host API, native-tool, and Web reads.
entry_points: Loop action execution; `compozy task run result`; GET /api/task-runs/:id/result; tasks/runs/result; compozy__task_run_result; Web task result
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260831-181713-956331-lab/qa-artifacts/qa/result-post-restart.sha256; /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260831-181713-956331-lab/qa-artifacts/qa/oversized-task-runs.json
last_report: docs/qa/reports/2026-08-31-loop-result-fix.md
overlaps: LP-run-agent-output-ownership; TA-action-run-liveness
---

Run one action that deterministically returns more than 64 KiB but remains within the effective action-result budget. Confirm it succeeds, the task-run envelope carries only `result_ref` and `result_bytes`, and exact bytes remain readable after daemon restart. Then run the same action above the effective budget and confirm the task and node fail with `action_result_too_large`, the lease is no longer active, no successful output is published, and the authored Loop failure policy remains recoverable.

QA 2026-08-20: a 70,000-byte builtin transform completed because that executor externalizes its result before the raw action-result boundary, so it was excluded as the wrong path. Human rerun: install a resource extension action that returns more than 64 KiB directly, execute it in a Loop, then confirm CLI and HTTP show a failed task/run, no active lease, no success output, and one bounded validation error.

QA impact 2026-08-31: the 64 KiB failure contract was removed. Loop action values now use the effective result budget and durable result references; this scenario was reset for the new success and failure boundaries.

QA 2026-08-31: a 71,694-byte transform completed with only `result_ref` and `result_bytes` in the run envelope, then reproduced the same SHA-256 after daemon restart. A 307,200-byte transform exceeded the 256 KiB action budget and failed as `action_result_too_large` with the node, action kind, actual bytes, limit, and recovery guidance; it published no result and retained no lease.
