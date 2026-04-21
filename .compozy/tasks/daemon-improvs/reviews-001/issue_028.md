---
status: resolved
file: internal/store/rundb/run_db.go
line: 156
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58go85,comment:PRRC_kwDORy7nkc651UNl
---

# Issue 028: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Only nil out `r.db` after a successful close.**

If `closeRunSQLiteDatabase` fails, the SQLite handle may still be open but `RunDB` is already marked closed. That makes retries impossible and can hide a leaked connection from callers.  


<details>
<summary>Suggested fix</summary>

```diff
 func (r *RunDB) CloseContext(ctx context.Context) error {
 	if r == nil || r.db == nil {
 		return nil
 	}
 	if ctx == nil {
 		return errors.New("rundb: close context is required")
 	}
 	db := r.db
-	r.db = nil
-	return closeRunSQLiteDatabase(ctx, db)
+	if err := closeRunSQLiteDatabase(ctx, db); err != nil {
+		return err
+	}
+	r.db = nil
+	return nil
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/rundb/run_db.go` around lines 147 - 156, The CloseContext
method currently sets r.db = nil before calling closeRunSQLiteDatabase, which
hides failures; change it so you capture the db handle into a local variable (db
:= r.db), call closeRunSQLiteDatabase(ctx, db) first, check its error, and only
on success set r.db = nil and return nil; on error leave r.db intact and return
the error. Update the CloseContext implementation (referencing CloseContext,
r.db, and closeRunSQLiteDatabase) to nil out the field only after a successful
close.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5db983a7-a2cf-4801-8b6f-2aa2e3a5e394 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `RunDB.CloseContext` nils `r.db` before `closeRunSQLiteDatabase` returns. If the close fails, callers lose access to the still-open handle and cannot retry cleanup.
- Fix approach: serialize close attempts, keep `r.db` intact on failure, clear it only after a successful close, and add regression coverage for retry-after-failure behavior.
- Resolution: `RunDB.CloseContext` now serializes close attempts with `closeMu`, preserves the cached handle on failure, and clears it only after a successful SQLite close.
- Verification: `go test ./internal/store/rundb` and `make verify`
