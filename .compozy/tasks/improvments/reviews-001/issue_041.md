---
status: resolved
file: internal/store/globaldb/registry.go
line: 426
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUl4,comment:PRRC_kwDORy7nkc68K-Q-
---

# Issue 041: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Make the empty-workspace delete atomic.**

`WorkspaceCatalogStats()` and the `DELETE` are separate operations, so a workflow/run can be inserted between them and this method can still remove the workspace row. Fold the emptiness check into the `DELETE` itself or execute both steps in one transaction.

<details>
<summary>💡 Proposed fix</summary>

```diff
-	stats, err := g.WorkspaceCatalogStats(ctx, workspaceID)
-	if err != nil {
-		return false, err
-	}
-	if stats.WorkflowCount > 0 || stats.RunCount > 0 {
-		return false, nil
-	}
-
-	result, err := g.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, workspaceID)
+	result, err := g.db.ExecContext(
+		ctx,
+		`DELETE FROM workspaces
+		 WHERE id = ?
+		   AND NOT EXISTS (SELECT 1 FROM workflows WHERE workspace_id = ?)
+		   AND NOT EXISTS (SELECT 1 FROM runs WHERE workspace_id = ?)`,
+		workspaceID,
+		workspaceID,
+		workspaceID,
+	)
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/globaldb/registry.go` around lines 410 - 426, The current code
calls WorkspaceCatalogStats(...) then issues a separate DELETE, which is racy;
instead make the deletion atomic by folding the emptiness check into the DELETE
itself (or run both operations inside a single transaction). Replace the
two-step approach around g.db.ExecContext(`DELETE FROM workspaces WHERE id = ?`,
...) and WorkspaceCatalogStats by issuing a single DELETE that only removes the
workspace when no workflows or runs exist (e.g. DELETE FROM workspaces WHERE id
= ? AND NOT EXISTS (SELECT 1 FROM workflows WHERE workspace_id = ?) AND NOT
EXISTS (SELECT 1 FROM runs WHERE workspace_id = ?)), then inspect
result.RowsAffected() to determine success; alternatively, begin a transaction
with g.db.BeginTx, call WorkspaceCatalogStats within that tx and delete within
the same tx before commit to ensure atomicity.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4021176c-ee24-47e4-a6f3-a97e632a8084 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes: `DeleteWorkspaceIfNoCatalogData` checked `WorkspaceCatalogStats` before issuing an unconditional `DELETE`, leaving a race where catalog rows could be inserted between the check and delete. The fix is to fold both emptiness predicates into the `DELETE` statement and use `RowsAffected` for the result.
