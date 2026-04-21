---
status: resolved
file: internal/store/rundb/run_db.go
line: 167
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58ixs1,comment:PRRC_kwDORy7nkc654MFj
---

# Issue 006: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Don’t keep `r.db` cached after a failed close that already closed the handle.**

At Line 164, `closeRunSQLiteDatabase` can return an error even after the underlying `*sql.DB` has been closed. `internal/store/sqlite.go` does the WAL checkpoint and the handle close in the same call, so a checkpoint failure here leaves `r.db` non-`nil` but unusable. That turns later retries into permanent `"database is closed"` failures instead of a real retry path.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/rundb/run_db.go` around lines 163 - 167,
closeRunSQLiteDatabase(ctx, db) can return an error even though the underlying
*sql.DB has been closed; to avoid leaving r.db pointing at a closed/unusable
handle, call closeRunSQLiteDatabase(ctx, db), capture its error, then
unconditionally set r.db = nil before returning (preserve and return the error).
Update the block surrounding r.db and closeRunSQLiteDatabase to always clear the
cached r.db regardless of the error path so retries don't hit "database is
closed" permanently.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e40d238e-ad07-4f04-8bea-f476de16b781 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `store.CloseSQLiteDatabase` checkpoints and closes in one call, and it can return an error after the underlying `*sql.DB` has already been closed. `RunDB.CloseContext` currently preserves `r.db` on that error path, which can leave the cached handle unusable for all later callers.
- Plan: always clear `r.db` after invoking `closeRunSQLiteDatabase`, return the close error if any, and update the close-path tests to reflect the real lower-level semantics.
- Resolution: changed `internal/store/rundb/run_db.go` to clear `r.db` before returning the result of `closeRunSQLiteDatabase`, so callers do not retain a closed handle after checkpoint-related close failures.
- Regression coverage: updated `internal/store/rundb/close_test.go` so the failing-close case now asserts the cached handle is cleared immediately and no second close attempt is made against a dead `*sql.DB`.
- Verification: `go test ./internal/store/rundb -run 'Test(RunDBCloseContext|RunDBUpsertIntegritySerializesConcurrentMerges)$' -count=1` passed. `make verify` then passed with `2544` tests and `2` skipped helper-process tests.
