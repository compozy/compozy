---
status: resolved
file: internal/core/model/model_test.go
line: 128
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHB5,comment:PRRC_kwDORy7nkc60y0f3
---

# Issue 008: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
# Quick check: each Test* function should contain t.Run("Should...") blocks.
awk '
/^func Test/ {
  if (fn != "") { print fn ": " (hasRun ? "has t.Run" : "NO t.Run") }
  fn=$2; hasRun=0; next
}
fn != "" && /t\.Run\("Should/ { hasRun=1 }
END {
  if (fn != "") { print fn ": " (hasRun ? "has t.Run" : "NO t.Run") }
}
' internal/core/model/model_test.go
```

Repository: compozy/compozy

Length of output: 320

---



</details>

**Tests must follow the required `t.Run("Should...")` subtest pattern consistently.**

Five of the six test functions assert directly in the function body without subtests. Convert them to table-driven subtests using `t.Run("Should...")` naming.

Additionally, `TestIsActiveWorkflowDirName` already uses subtests but must rename them to follow the "Should..." pattern (e.g., `t.Run("Should return true for regular workflow names", ...)`).

<details>
<summary>Example structure to apply</summary>

```diff
 func TestRuntimeConfigApplyDefaults(t *testing.T) {
 	t.Parallel()
-
-	cfg := &model.RuntimeConfig{}
-	cfg.ApplyDefaults()
-	...
+	testCases := []struct {
+		name string
+	}{
+		{name: "Should apply defaults for empty config"},
+	}
+	for _, tc := range testCases {
+		tc := tc
+		t.Run(tc.name, func(t *testing.T) {
+			t.Parallel()
+			cfg := &model.RuntimeConfig{}
+			cfg.ApplyDefaults()
+			...
+		})
+	}
 }
```
</details>

This is a critical pattern per coding guidelines: "MUST use t.Run("Should...") pattern for ALL test cases."

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/model/model_test.go` around lines 11 - 128, Convert the direct
assertions into t.Run("Should ...") subtests and make existing subtests follow
the "Should..." naming: for TestRuntimeConfigApplyDefaults, wrap each
expectation (concurrent, batch size, IDE, reasoning effort, mode, timeout, retry
multiplier) as individual t.Run subtests (call cfg.ApplyDefaults() once in a
parent t.Run or setup and run assertions in subtests) and call t.Parallel()
inside each subtest; for TestPathHelpers, create t.Run subtests for
TasksBaseDir, TaskDirectory, and ArchivedTasksDir checks; for TestJobIssueCount,
wrap the IssueCount assertion in a t.Run subtest; for
TestUsageTotalUsesExplicitTotalWhenPresent, create two t.Run subtests for
explicit TotalTokens present and absent cases; for
TestRuntimeConfigApplyDefaultsPreservesExplicitValues, wrap the preservation
check in a t.Run subtest; and in TestIsActiveWorkflowDirName rename each case’s
t.Run to the "Should..." format (e.g., "Should return true for regular workflow
names") and keep t.Parallel() in each subtest. Use the existing symbols
(RuntimeConfig.ApplyDefaults, TasksBaseDir, TaskDirectory, ArchivedTasksDir,
IsActiveWorkflowDirName, Job.IssueCount, Usage.Total) to locate and modify the
tests.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:bdf3a9fe-87dc-4393-b204-a681baf5669a -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Rationale: The touched test file still has several top-level assertion blocks without `t.Run("Should ...")` subtests, and one existing subtest table still uses non-`Should...` names. That does not match the repository's explicit test-structure rule.
- Fix plan: Restructure the affected tests into named `t.Run("Should ...")` subtests and rename the existing workflow-name table cases accordingly.
- Resolution: Reworked the affected model tests into `t.Run("Should ...")` subtests and renamed the workflow-directory table cases to match the required convention. `make verify` passed.
