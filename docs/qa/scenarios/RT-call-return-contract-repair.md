---
id: RT-call-return-contract-repair
area: RT
title: Admit a child's result against its contract with one repair round
persona: Bruno
journey: J-delegate-work-to-an-agent
expected: A conforming return settles completed in one transaction, a violating return gets the sanitized validator errors verbatim and exactly one repair attempt before settling invalid-result with both tries kept, and a contracted child that admits nothing settles completed-without-result.
entry_points: compozy call reviewer "Return a verdict" --expect @review-contract.json --strict; compozy__call_return with {"result":{"verdict":"needs-changes"}}; compozy call show call_01JBD8G2K7Q9; compozy call result call_01JBD8G2K7Q9; HTTP and UDS GET /api/workspaces/{workspace_id}/calls/{call_id}/result
qa_status: fail
bug_ids: BUG-20260826-call-child-tool-policy
fix_status: fixed
retest_status: pending
fix_commits: 5df9697
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/qa-remediation-public-retest.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-agent-call-golden-path; TA-task-result-contract; LP-loop-contract-regime-adoption
---

`compozy__call_return` is the child's terminal act, and recording the result and settling the call
are one act, not two. Prove the transaction boundary first: a `completed` state row with a missing
or invalid result reference must be impossible, so kill the daemon mid-return and confirm the call
is either untouched or wholly settled — never half.

Then walk the repair round. The first violation returns the validator's errors **verbatim from the
already-sanitized payload** and grants exactly one more attempt; a second failure settles
`invalid-result` with both attempts' errors recorded and readable. Confirm an infrastructure failure
never consumes the repair attempt, and that a single-key wrapper around an otherwise valid payload
is unwrapped rather than failed.

Cover the verdict provenance too — `returned`, `extracted` and `repaired` must each be visible on
the record rather than flattened into one word, `--strict` must disable prose extraction so an
uncontracted-but-contracted child settles `completed-without-result` directly, and a return against
an already-settled call must fail `call_already_settled` while a return with no bound call fails
`call_return_unbound`. Finish by confirming the contract digest on the record is the one pinned at
create time and that the example-shape shorthand and its expanded schema pin the same digest.
