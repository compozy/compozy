---
id: RT-agent-call-golden-path
area: RT
title: Complete a typed agent call from creation through result fetch
persona: Bruno
journey: J-delegate-work-to-an-agent
expected: A typed call is accepted asynchronously, await reaches completed, and result returns the complete admitted JSON without losing profile or workspace ownership.
entry_points: compozy call reviewer "Review HEAD~1..HEAD" --expect @review-contract.json --idempotency-key rev-01; compozy call await call_01JBD8G2K7Q9 --timeout 120s; compozy call result call_01JBD8G2K7Q9; compozy call show call_01JBD8G2K7Q9; compozy call list --state running,completed --limit 3; HTTP and UDS POST /api/workspaces/{workspace_id}/calls with {"target":{"agent":"reviewer"},"prompt":"Review HEAD~1..HEAD","expect":{},"idempotency_key":"rev-01"}; HTTP and UDS GET /api/workspaces/{workspace_id}/calls/{call_id} and /result; compozy__agent_call with {"agent":"reviewer","prompt":"Review HEAD~1..HEAD","expect":{},"idempotency_key":"rev-01"}; compozy__call_await with {"call_ids":["call_01JBD8G2K7Q9"],"timeout_ms":120000}; compozy__call_result with {"call_id":"call_01JBD8G2K7Q9"}
qa_status: fail
bug_ids: BUG-20260826-operator-caller-model-runtime; BUG-20260826-call-child-tool-policy
fix_status: fixed
retest_status: pending
fix_commits: 82d27bca1; 5df9697
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/operator-caller-runtime-reproduction.md; /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/qa-remediation-public-retest.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-call-return-contract-repair; RT-call-wake-delivery-exactly-once; RT-call-profile-scope-isolation
---

Walk the create → await → result flow in both Global and workspace scope, then compare the CLI and HTTP projections.

This row also owns the idempotency fence, because the golden path is where a caller most plausibly
retries. Replay the same `--idempotency-key` and confirm the original call id comes back with
`replayed: true`; fire two concurrent creates carrying the same key and confirm the database
uniqueness fence on profile, scope, workspace, caller and key resolves them to exactly one call row
rather than two; then replay the key with a different result budget or deadline and confirm
`call_idempotency_conflict` naming the original call id, since budget and deadline participate in
idempotency identity.
