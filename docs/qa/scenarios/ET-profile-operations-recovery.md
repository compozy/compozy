---
id: ET-profile-operations-recovery
area: ET
title: Recover interrupted profile lifecycle operations
persona: Ada
journey: J-operate-profiles
expected: Interrupted rename, archive, or delete work remains a durable lifecycle operation with a stable step and redacted error; boot recovery converges safe operations, terminal failure remains inspectable, and retry resumes without duplicating committed effects.
entry_points: compozy profile ops; compozy profile ops retry; GET /api/profiles/ops; POST /api/profiles/ops/{op_id}/retry; profile.lifecycle_op_recovered|failed events
qa_status: pass
bug_ids: BUG-20261003-profile-archive-event-rejected; BUG-20261003-profile-unavailable-identity-write; BUG-20261003-profile-recovery-blank-desktop
fix_status: fixed
retest_status: pass
fix_commits: fe8a644b1; 664f24775
evidence: docs/qa/evidence/2026-10-02-untested/profile-recovery-final-ada-ended.json; docs/qa/evidence/2026-10-02-untested/profile-recovery-rename-crash-proof.json; docs/qa/evidence/2026-10-02-untested/profile-recovery-delete-crash-proof.json; docs/qa/evidence/2026-10-02-untested/profile-recovery-archive-crash-proof.json; docs/qa/evidence/2026-10-02-untested/profile-recovery-boundary-ada-ended.json
last_report: docs/qa/reports/2026-10-02-untested.md
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

QA 2026-10-03: a real destination-directory conflict leaves an inspectable failed rename.
CLI/HTTP/UDS agree, restart leaves the failure untouched, selection and duplicate names refuse,
and explicit retry after preserving the conflicting import completes without losing content.
The failure audit is missing from public logs; identity updates also bypass the reservation.
Both findings are registered. Applied/finalizing crash recovery and the complete scenario remain
Pending; the bounded failure/retry walk does not establish those branches.

Final repair replay: failure/recovery and archived-identity audits remain durable, and identity
edits refuse an unfinished operation. Real rename and delete SIGKILL interruptions recover on
boot; all 2,048 authored files retain their bytes before deletion. Another clean restart preserves
one recovery event per operation without replaying done work. These two bugs are verified.
The scenario remains fail/pending for the separate blank desktop finding and the unwalked archive
interruption. No complete recovery-charter verdict is inferred from these bounded passes.

The blank-desktop repair now passes the real-daemon E2E and fresh Ada Chrome replay. Cold entry
shows the reserved owner, exact operation and CLI remedy without changing selection. Explicitly
switching to default opens Settings; correcting the conflict and retrying restores the original
profile, and its Settings opens and survives reload. All three filed findings are now verified.
Archive interruption still prevents the complete recovery verdict; final scenario reconciliation
follows that missing branch. Evidence: profile-recovery-blank-final-ada-ended.json.

Final-source verification closes this scenario: the recovery status retains the exact operation
and remedy without an adapter import cycle, and the fresh Ada Chrome replay preserves explicit
selection and content through retry/reload. Archive operation op_01M416R7J0VXJAM1BD61JB1GYN is
interrupted by SIGKILL at finalizing/reconcile_automations. Boot completes it; CLI, HTTP and UDS
agree. All 64 planned monthly jobs retain their definitions and remain paused, including after
unarchive. The default-owned recovery event sum-114aa8854754d580 remains unique after another
clean boot; repeating archive returns the same paused IDs. Retrying the completed operation is
correctly refused as profile_op_not_retryable. The helper's initial contrary assumption is recorded
as a driver correction. No product source was read during this walk. The separate TA-052 cursor
failure remains open and is not qualified by the larger archive inventory page.
