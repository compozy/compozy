---
provider: coderabbit
pr: "133"
round: 3
round_created_at: 2026-04-30T21:50:44.830324Z
status: resolved
file: internal/store/globaldb/read_queries_test.go
line: 265
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-3Yfs,comment:PRRC_kwDORy7nkc69AEIE
---

# Issue 011: _⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_

**Wrap this test case in a `t.Run("Should...")` subtest.**

The new case is good, but it should follow the enforced subtest convention for test cases in this repo.

<details>
<summary>Suggested adjustment</summary>

```diff
 func TestReadQueriesBulkWorkflowSummariesByID(t *testing.T) {
 	t.Parallel()
+	t.Run("Should return bulk task counts and archive eligibility for mixed workflows", func(t *testing.T) {
+		t.Parallel()

-	db := openTestGlobalDB(t)
+		db := openTestGlobalDB(t)
 	defer func() {
 		if err := db.Close(); err != nil {
 			t.Errorf("close test global db: %v", err)
 		}
 	}()
@@
 	if !resolvedEligibility.Archivable() {
 		t.Fatalf("resolved eligibility should be archivable: %#v", resolvedEligibility)
 	}
+	})
 }
```
</details>

 
As per coding guidelines, `**/*_test.go`: “MUST use t.Run("Should...") pattern for ALL test cases”.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/globaldb/read_queries_test.go` around lines 181 - 265, Wrap
the TestReadQueriesBulkWorkflowSummariesByID test body in a t.Run subtest using
the "Should..." naming pattern (e.g. t.Run("Should return correct workflow
summary eligibility and counts", func(t *testing.T) { ... })); move the existing
t.Parallel() and all setup/assertions into that subtest closure so the test
follows the repo's subtest convention while preserving behavior of functions
like openTestGlobalDB, mustWorkspace, mustReconcileArchiveWorkflow,
TaskCountsByWorkflowIDs, and WorkflowArchiveEligibilityByIDs.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:d0f092d4-f42b-498e-943a-5993321058fa -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `TestReadQueriesBulkWorkflowSummariesByID` is still a standalone top-level test body rather than the required `t.Run("Should...")` structure.
- Fix approach: wrap the full test body in one descriptive `Should...` subtest and preserve the current setup and assertions.
- Resolution: wrapped the workflow summary test in a `Should...` subtest and verified the batch with targeted tests plus `make verify`.
