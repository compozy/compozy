---
status: resolved
file: internal/cli/commands_test.go
line: 20
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUlJ,comment:PRRC_kwDORy7nkc68K-QA
---

# Issue 013: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Wrap this updated case in `t.Run("Should ...")` to match test policy.**

Please keep modified tests aligned with the required subtest naming/pattern.

<details>
<summary>♻️ Suggested refactor</summary>

```diff
 func TestBuildConfigTasksRunAlwaysEnablesExecutableExtensions(t *testing.T) {
 	t.Parallel()
-
-	state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
-
-	cfg, err := state.buildConfig()
-	if err != nil {
-		t.Fatalf("buildConfig: %v", err)
-	}
-	if !cfg.EnableExecutableExtensions {
-		t.Fatal("expected tasks run config to enable executable extensions")
-	}
+	t.Run("Should enable executable extensions for tasks run", func(t *testing.T) {
+		t.Parallel()
+		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
+		cfg, err := state.buildConfig()
+		if err != nil {
+			t.Fatalf("buildConfig: %v", err)
+		}
+		if !cfg.EnableExecutableExtensions {
+			t.Fatal("expected tasks run config to enable executable extensions")
+		}
+	})
 }
```
</details>

  
As per coding guidelines, `**/*_test.go`: MUST use `t.Run("Should...")` pattern for ALL test cases.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestBuildConfigTasksRunAlwaysEnablesExecutableExtensions(t *testing.T) {
	t.Parallel()
	t.Run("Should enable executable extensions for tasks run", func(t *testing.T) {
		t.Parallel()
		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		cfg, err := state.buildConfig()
		if err != nil {
			t.Fatalf("buildConfig: %v", err)
		}
		if !cfg.EnableExecutableExtensions {
			t.Fatal("expected tasks run config to enable executable extensions")
		}
	})
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/commands_test.go` around lines 9 - 20, Wrap the assertions in
TestBuildConfigTasksRunAlwaysEnablesExecutableExtensions into a t.Run subtest
named with the "Should ..." pattern (e.g. t.Run("Should enable executable
extensions for tasks run") { ... }), keeping the setup (state :=
newCommandState(...)), the call to state.buildConfig(), and the checks against
cfg.EnableExecutableExtensions inside the subtest block so the test file follows
the required t.Run("Should...") pattern while preserving the existing logic and
error handling.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:0aa58313-6e22-4f13-a85f-3db5cd1d7a6e -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes: The tasks-run executable-extension test did not use the required subtest pattern. Wrapped the assertions in `t.Run("Should enable executable extensions for tasks run", ...)`.
