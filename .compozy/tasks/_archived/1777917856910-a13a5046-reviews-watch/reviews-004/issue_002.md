---
provider: coderabbit
pr: "133"
round: 4
round_created_at: 2026-04-30T22:06:27.568795Z
status: resolved
file: internal/core/sync_test.go
line: 182
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-36gZ,comment:PRRC_kwDORy7nkc69AyBP
---

# Issue 002: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Use `t.Run("Should...")` for the newly added single-scenario test cases.**

These new tests are valid, but they bypass the required subtest naming pattern in this repo. Please wrap each scenario in a `t.Run("Should...")` block for consistency and policy compliance.

<details>
<summary>Suggested pattern (example)</summary>

```diff
 func TestSyncTaskMetadataSkipsEmptyReviewDirectories(t *testing.T) {
-	workspaceRoot := t.TempDir()
-	setSyncTestHome(t)
+	t.Run("Should skip empty review directories", func(t *testing.T) {
+		workspaceRoot := t.TempDir()
+		setSyncTestHome(t)

-	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "empty-review-demo")
-	writeSyncWorkflowFile(t, workflowDir, "task_01.md", taskBody("pending", "Demo task"))
-	if err := os.MkdirAll(filepath.Join(workflowDir, "reviews-002"), 0o755); err != nil {
-		t.Fatalf("mkdir empty reviews dir: %v", err)
-	}
+		workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "empty-review-demo")
+		writeSyncWorkflowFile(t, workflowDir, "task_01.md", taskBody("pending", "Demo task"))
+		if err := os.MkdirAll(filepath.Join(workflowDir, "reviews-002"), 0o755); err != nil {
+			t.Fatalf("mkdir empty reviews dir: %v", err)
+		}

-	result, err := Sync(context.Background(), SyncConfig{TasksDir: workflowDir})
-	if err != nil {
-		t.Fatalf("Sync(): %v", err)
-	}
-	if result.WorkflowsScanned != 1 || result.ReviewRoundsUpserted != 0 || result.ReviewIssuesUpserted != 0 {
-		t.Fatalf("unexpected sync result: %#v", result)
-	}
+		result, err := Sync(context.Background(), SyncConfig{TasksDir: workflowDir})
+		if err != nil {
+			t.Fatalf("Sync(): %v", err)
+		}
+		if result.WorkflowsScanned != 1 || result.ReviewRoundsUpserted != 0 || result.ReviewIssuesUpserted != 0 {
+			t.Fatalf("unexpected sync result: %#v", result)
+		}
+	})
 }
```
</details>

 

As per coding guidelines, "**MUST use t.Run("Should...") pattern for ALL test cases**".


Also applies to: 184-217, 344-370, 530-568, 666-688

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/sync_test.go` around lines 115 - 182, The test function
TestSyncTaskMetadataRootScanPrunesDeletedWorkflowRows and the other newly added
single-scenario tests must be converted into subtests that follow the repo
pattern by wrapping their assertions in t.Run("Should <describe behavior>",
func(t *testing.T) { ... }), so edit the body of
TestSyncTaskMetadataRootScanPrunesDeletedWorkflowRows (and the other affected
tests at the noted ranges) to move setup/assertion code into a single t.Run call
with a "Should..." description, preserving the existing calls to Sync, sqlDB
queries, and helper funcs (writeSyncWorkflowFile, openSyncSQLite, queryCount)
and returning the same failures using t.Fatalf inside the subtest. Ensure the
top-level Test* function remains and only contains one t.Run subtest per
scenario with the original test logic.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:6b41ddd9-f24d-40b8-b3d3-198b80979b9a -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The cited functions are all single-scenario tests that place setup/assertions directly in the top-level `Test*` body instead of the repository-standard `t.Run("Should ...")` subtest wrapper.
  - Root cause: the recently added regression tests followed a flat style that is inconsistent with the local test conventions already used across `internal/core`.
  - Fix approach: keep the top-level `Test*` functions, wrap each single scenario in one `t.Run("Should ...")` block, and preserve the existing assertions and helper usage inside the subtest.
  - Verification: `go test ./internal/core` passed during focused validation, and `make verify` passed after the final patch.
