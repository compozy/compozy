---
status: resolved
file: internal/store/rundb/run_db.go
line: 810
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58go9A,comment:PRRC_kwDORy7nkc651UNw
---

# Issue 029: _⚠️ Potential issue_ | _🔴 Critical_
## Review Comment

_⚠️ Potential issue_ | _🔴 Critical_

**Make the integrity merge atomic.**

This is a read-modify-write sequence over the singleton row. Two callers can read the same old state, merge different reasons, and the last writer wins, which drops previously detected integrity signals. Please do the read + merge + upsert in a single transaction, or move the merge semantics into SQL.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/rundb/run_db.go` around lines 759 - 810, The read-modify-write
race happens because GetIntegrity() and the subsequent upsert (r.db.ExecContext
with ON CONFLICT) are separate operations; wrap the read + merge + upsert in a
single database transaction to make the merge atomic. Specifically, start a
transaction (e.g., tx, err := r.db.BeginTx(ctx, nil)), call a transactional
variant of GetIntegrity that selects the singleton row inside the tx (using
tx.QueryRowContext or SELECT ... FOR UPDATE if supported), perform the same
merge into RunIntegrityState (using r.now() and encodeIntegrityReasons), then
execute the INSERT ... ON CONFLICT upsert using tx.ExecContext and commit the
tx; ensure errors roll back the tx. This keeps the existing logic
(mergeIntegrityReasons, encodeIntegrityReasons, r.now, store.FormatTimestamp)
but performs all operations using the tx instead of r.db to prevent lost
updates.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5db983a7-a2cf-4801-8b6f-2aa2e3a5e394 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `UpsertIntegrity` performs `GetIntegrity` and the final upsert as separate unsynchronized operations. Concurrent callers can merge against the same old row and overwrite each other's newly discovered integrity reasons.
- Fix approach: make the integrity update atomic by serializing the merge path and executing the read/merge/upsert within one transaction, then add a regression test that exercises concurrent updates and verifies the merged reasons persist together.
- Resolution: `UpsertIntegrity` now serializes the merge path with `integrityMu`, performs the read/merge/upsert inside one transaction, and `TestRunDBUpsertIntegritySerializesConcurrentMerges` covers concurrent updates.
- Verification: `go test ./internal/store/rundb` and `make verify`
