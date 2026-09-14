---
id: ET-profile-cli-lifecycle
area: ET
title: Manage a profile through its complete CLI lifecycle
persona: Ada
journey: J-operate-profiles
expected: Create, update, rename, archive, unarchive, and delete use daemon-owned profile state; every planned mutation applies exactly the previewed revision, preserves or removes the documented ownership rows, and returns matching human and structured results.
entry_points: compozy profile list|current|create|update|rename|archive|unarchive|delete; local HTTP/UDS /api/profiles routes
qa_status: fail
bug_ids: BUG-20260914-profile-rename-mcp-reference
fix_status: pending
retest_status:
fix_commits:
evidence:
last_report:
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
Vault/SQLite lifecycle suite passed; the corresponding CLI walk remains pending.
