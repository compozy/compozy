---
id: ET-profile-operations-recovery
area: ET
title: Recover interrupted profile lifecycle operations
persona: Ada
journey: J-operate-profiles
expected: Interrupted rename, archive, or delete work remains a durable lifecycle operation with a stable step and redacted error; boot recovery converges safe operations, terminal failure remains inspectable, and retry resumes without duplicating committed effects.
entry_points: compozy profile ops; compozy profile ops retry; GET /api/profiles/ops; POST /api/profiles/ops/{op_id}/retry; profile.lifecycle_op_recovered|failed events
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-cli-lifecycle
---

Flagged by Profiles task 04. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. Interrupt each multi-step lifecycle mutation at a supported fault boundary and restart the daemon.
2. Inspect the operation through CLI, HTTP, and UDS; compare id, kind, profile, status, step, and error.
3. Prove automatic recovery completes safe pending work and emits `profile.lifecycle_op_recovered`.
4. Preserve one terminal failure, correct its cause, retry by operation ID, and prove already committed
   effects are not duplicated.
5. Confirm operation errors and events contain no secret value or Vault reference.

Expected evidence: fault-injection transcript, pre/post-restart operation payloads, exact lifecycle
events, side-effect counts, and the successful retry result.
