---
id: ET-profile-cli-lifecycle
area: ET
title: Manage a profile through its complete CLI lifecycle
persona: Ada
journey: J-operate-profiles
expected: Create, update, rename, archive, unarchive, and delete use daemon-owned profile state; every planned mutation applies exactly the previewed revision, preserves or removes the documented ownership rows, and returns matching human and structured results.
entry_points: compozy profile list|current|create|update|rename|archive|unarchive|delete; local HTTP/UDS /api/profiles routes
qa_status: fail
bug_ids: BUG-20261003-profile-delete-orphans-automations; BUG-20261003-profile-archive-resource-automations; BUG-20260914-profile-rename-mcp-reference
fix_status: pending
retest_status: pending
fix_commits: c131f5764; b4ed8ca18; 74744060b
evidence: docs/qa/reports/2026-09-14-marketplace-review-public.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-profile-selection-precedence; ET-profile-operations-recovery; ET-profile-lifecycle-race-guards; ET-profile-approval-owner-resume
---

Flagged by Profiles task 04. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. Create and activate a profile with explicit identity, then confirm list/current parity in human,
   JSON, JSONL, and TOON output.
2. Update its identity and rename it with both `--repos none` and selected repository-folder effects;
   compare the prepare plan, applied effects, Vault ref rewrites, and result field for field.
3. Archive it after creating paused and queued fixtures; prove running/approval guards, selection
   unavailability, frozen queued work, and idempotent repeat behavior.
4. Unarchive it and prove frozen work is claimable while paused automations stay paused.
5. Remove every owned work root, inspect the delete preview, delete with `--yes`, and prove the result
   equals the preview, remembered selections are swept, and the name can be created again.

Expected evidence: structured transcripts for every verb and plan, ownership counts before/after,
Vault rewrite records, lifecycle events, and the final name-reuse result.

PR636 review retest: seed manual and extension MCP tokens, registrations and configured client
secrets in both profile and workspace-profile cells. Rename must enumerate every ref occurrence,
keep tokens and registered client secrets decryptable under the new profile identity, and remove
old refs. Deletion must count and remove all exclusive credential rows while preserving shared,
user-scoped and other-profile credentials. Compare preview and actual removal counts. The real
Vault/SQLite lifecycle suite passed; the targeted CLI walk is recorded below.

PR636 targeted result 2026-09-14: Both configured profile layers and all 20 standalone credential refs across four owner cells renamed and deleted through public CLI with exact preview/apply counts and protected refs retained; encrypted OAuth rows remain owning integration evidence. See docs/qa/reports/2026-09-14-marketplace-review-public.md. Broader historical scenario steps were not rerun in this targeted pass.

QA 2026-10-03 archive walk: the enabled resource job is absent from HTTP/UDS plans and remains
scheduler-registered after Web archive. Unarchive reports no paused automations and retains the
enabled job. The running-session blocker and cancellation behave correctly. Shared backend defect
BUG-20261003-profile-archive-resource-automations is open; fresh repair replay is pending. See
profile-archive-sol-* receipts and the dated report. No actual automation execution is claimed.

QA 2026-10-03 archive repair replay: the enabled job and trigger now appear in the plan and pause
on archive. Unarchive retains both pauses; per-item keyboard reactivation uses the actual owner.
CLI repeat preserves the exact audit list, and a real daemon restart keeps both definitions paused.
BUG-20261003-profile-archive-resource-automations is verified. Evidence: profile-archive-fixed-sol-*
and profile-archive-fixed-cli-* in the dated report. The complete lifecycle charter remains Pending.

QA 2026-10-03 deletion walk: a paused canonical job and trigger are omitted from work counts.
Settings labels their owner Empty and permits deletion, leaving both aggregate automation lists
broken by an absent owner. BUG-20261003-profile-delete-orphans-automations is open.
See profile-delete-navigation-sol-* and the dated report; full lifecycle verdict remains Pending.

QA 2026-10-03 deletion repair: canonical jobs/triggers now count as work, including paused
definitions. Web omits Delete for their archived owner, and direct CLI deletion refuses it.
An empty archived profile still deletes successfully with its exact revision; reload and UDS
confirm it is gone while both aggregate catalogs retain the other owner and its automations.
BUG-20261003-profile-delete-orphans-automations is verified; see profile-delete-fixed-sol-*
and the dated report. The full lifecycle charter remains Pending.
