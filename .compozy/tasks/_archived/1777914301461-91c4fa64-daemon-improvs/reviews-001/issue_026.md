---
status: resolved
file: internal/store/globaldb/global_db.go
line: 84
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58go8u,comment:PRRC_kwDORy7nkc651UNX
---

# Issue 026: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Do not permanently mark closed before the close operation succeeds.**

`closed` is flipped before checkpoint/close executes. If `closeGlobalSQLiteDatabase` fails once, later calls return early and never retry cleanup.

<details>
<summary>Proposed fix</summary>

```diff
 	if !g.closed.CompareAndSwap(false, true) {
 		return nil
 	}
-	return closeGlobalSQLiteDatabase(ctx, g.db)
+	if err := closeGlobalSQLiteDatabase(ctx, g.db); err != nil {
+		g.closed.Store(false)
+		return fmt.Errorf("globaldb: close sqlite database: %w", err)
+	}
+	return nil
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	if !g.closed.CompareAndSwap(false, true) {
		return nil
	}
	if err := closeGlobalSQLiteDatabase(ctx, g.db); err != nil {
		g.closed.Store(false)
		return fmt.Errorf("globaldb: close sqlite database: %w", err)
	}
	return nil
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/globaldb/global_db.go` around lines 80 - 84, The code
currently flips g.closed to true before calling closeGlobalSQLiteDatabase,
preventing retries on failure; change the logic so g.closed is only set to true
after closeGlobalSQLiteDatabase succeeds. Specifically, acquire a single-attempt
lock (either add a new atomic g.closing used with CompareAndSwap(false,true) to
guard concurrent closes, or keep the existing CompareAndSwap but if you set
g.closed to true only after err == nil), call closeGlobalSQLiteDatabase(ctx,
g.db), and if that call returns an error ensure the in-progress flag is cleared
(or do not set g.closed) so subsequent calls can retry; finally set g.closed to
true only on successful close.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:bc146171-8676-4e63-8078-ba462713e655 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `GlobalDB.CloseContext` flips `g.closed` before `closeGlobalSQLiteDatabase` succeeds. A close failure leaves the database handle open but permanently suppresses later retries because future calls return early.
- Fix approach: serialize close attempts, only mark the database closed after the SQLite close succeeds, and add regression coverage proving a failed close keeps the handle retryable.
- Resolution: `GlobalDB.CloseContext` now serializes close attempts with `closeMu`, returns the SQLite close error without marking the DB closed, and only sets `closed=true` after a successful close.
- Verification: `go test ./internal/store/globaldb` and `make verify`
