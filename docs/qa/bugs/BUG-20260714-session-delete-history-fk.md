# BUG-20260714-session-delete-history-fk: A stopped session with tool history cannot be deleted

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-11/J-complete-task-tree, delete a stopped session after real tool use
- **Scenarios:** RT-session-delete-owned-history
- **Found:** 2026-07-14 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** Cleanup of the live asynchronous Task activation replay

## Summary

The Delete session modal promised to remove the transcript and history, but every attempt to delete a stopped task-role session returned `Internal Server Error`. The failure persisted even after its terminal Task was deleted because the session still owned permission-log and token-stat rows.

## Reproduction

1. Run a real Cursor/Grok task-role session that invokes an AGH native tool.
2. Stop the session and delete its terminal Task.
3. Open the stopped session and confirm Delete session.
4. Retry through the public session DELETE endpoint.

**Expected:** The stopped session, transcript/catalog truth, permission history, and token statistics are removed atomically; unrelated sessions remain intact.
**Actual:** Both the UI and HTTP endpoint returned 500 with a SQLite foreign-key violation.

## Evidence

- Session `sess-12b2c865a27ecc72` had five `permission_log` rows before deletion; its modal and public DELETE both returned HTTP 500 before the fix.
- Historical pre-rebase evidence: the persisted v1 database upgraded in place to the then-current Automation schema version 2 with both existing history tables intact. In the merged stream, Hermes owns immutable v2 and the cascade is v3.
- The same modal then showed `Session deleted.`; a fresh public read returned 404 and database counts for the session were `sessions=0`, `permission_log=0`, and `token_stats=0`.

## Fix

- **Root cause:** `session_health` and `session_input_queue` declared session ownership with `ON DELETE CASCADE`, but `permission_log.session_id` and `token_stats.session_id` used restrictive foreign keys. Any real session with observability history therefore blocked its own catalog deletion. The deletion lifecycle also removed the catalog before the filesystem, so a later directory-removal failure could destroy owned permission/token history while leaving the session artifacts behind.
- **Correction:** Global migration v3 (`00003_schema.sql`) makes permission and token rows exact session-owned cascades; immutable v2 is the Hermes Bridge migration. Session deletion now atomically renames the directory to a tombstone before deleting catalog state, restores it if the catalog commit fails, and defers only post-commit tombstone cleanup. Startup retries retained tombstones; it never reconstructs partial audit history.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** `TestDeleteSessionRemovesDurableCatalogTruth/Should delete only the target session and its owned history` proves target cascades and foreign-session preservation. `TestGlobalMigrationPreservesSessionHistory` upgrades the exact previous embedded head, preserves representative permission/token values, reaches v3, and passes `PRAGMA foreign_key_check`. `TestManagerDelete` additionally proves directory rollback on catalog failure and logical success with a retained cleanup tombstone after a post-commit filesystem failure.

## Verification

- Production migration streams, schema equivalence, codegen check, focused deletion/status tests, and the complete GlobalDB package passed under `-race`.
- Historical pre-rebase lab evidence applied the then-current Automation migration v2 (`version=2`, `applied_count=2`) before the Browser retest; this observation is not the merged migration identity.
- The original failing real session was deleted successfully through the same confirmation modal; no manual database cleanup was used.
- Final Browser replay created `sess-6a4f5db74d195230` with Cursor/Grok 4.5, persisted one user prompt and one real response, stopped it, and deleted it through the confirmation modal. The route returned to the agent and rendered `Session deleted.` in 3.354 seconds with no foreign-key or partial-state error.
