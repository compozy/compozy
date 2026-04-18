---
status: resolved
file: internal/core/subprocess/process_unix_test.go
line: 221
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0ih,comment:PRRC_kwDORy7nkc624f71
---

# Issue 024: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Wrap this case in a `Should...` subtest.**

The behavior check is good, but the changed case should follow the required subtest pattern.



<details>
<summary>Proposed adjustment</summary>

```diff
 func TestLaunchUsesConfiguredWorkingDir(t *testing.T) {
 	t.Parallel()
-
-	workingDir := t.TempDir()
-	process, err := Launch(context.Background(), LaunchConfig{
-		Command:         shellCommand(t, "pwd"),
-		WorkingDir:      workingDir,
-		WaitErrorPrefix: "wait for test subprocess",
-	})
-	if err != nil {
-		t.Fatalf("launch process: %v", err)
-	}
-
-	output, err := io.ReadAll(process.Stdout())
-	if err != nil {
-		t.Fatalf("read stdout: %v", err)
-	}
-	if err := process.Wait(); err != nil {
-		t.Fatalf("wait process: %v", err)
-	}
-
-	got := strings.TrimSpace(string(output))
-	if got != workingDir {
-		t.Fatalf("pwd output = %q, want %q", got, workingDir)
-	}
+	t.Run("ShouldUseConfiguredWorkingDir", func(t *testing.T) {
+		t.Parallel()
+		workingDir := t.TempDir()
+		process, err := Launch(context.Background(), LaunchConfig{
+			Command:         shellCommand(t, "pwd"),
+			WorkingDir:      workingDir,
+			WaitErrorPrefix: "wait for test subprocess",
+		})
+		if err != nil {
+			t.Fatalf("launch process: %v", err)
+		}
+
+		output, err := io.ReadAll(process.Stdout())
+		if err != nil {
+			t.Fatalf("read stdout: %v", err)
+		}
+		if err := process.Wait(); err != nil {
+			t.Fatalf("wait process: %v", err)
+		}
+
+		got := strings.TrimSpace(string(output))
+		if got != workingDir {
+			t.Fatalf("pwd output = %q, want %q", got, workingDir)
+		}
+	})
 }
```
</details>

As per coding guidelines: `**/*_test.go`: MUST use `t.Run("Should...")` pattern for ALL test cases.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestLaunchUsesConfiguredWorkingDir(t *testing.T) {
	t.Parallel()
	t.Run("ShouldUseConfiguredWorkingDir", func(t *testing.T) {
		t.Parallel()
		workingDir := t.TempDir()
		process, err := Launch(context.Background(), LaunchConfig{
			Command:         shellCommand(t, "pwd"),
			WorkingDir:      workingDir,
			WaitErrorPrefix: "wait for test subprocess",
		})
		if err != nil {
			t.Fatalf("launch process: %v", err)
		}

		output, err := io.ReadAll(process.Stdout())
		if err != nil {
			t.Fatalf("read stdout: %v", err)
		}
		if err := process.Wait(); err != nil {
			t.Fatalf("wait process: %v", err)
		}

		got := strings.TrimSpace(string(output))
		if got != workingDir {
			t.Fatalf("pwd output = %q, want %q", got, workingDir)
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

In `@internal/core/subprocess/process_unix_test.go` around lines 196 - 221, Wrap
the entire test body of TestLaunchUsesConfiguredWorkingDir into a subtest using
t.Run with a descriptive name like "Should use configured working dir"; move
creation of workingDir, the Launch(...) call, reading stdout, process.Wait(),
and assertions into the subtest closure and call t.Parallel() inside that
closure so the subtest runs in parallel; keep references to Launch,
LaunchConfig, shellCommand, process.Stdout(), and process.Wait() unchanged
except for their new scope inside the t.Run closure.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:e644adda-6e52-4c35-ad45-842342f24cf4 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `TestLaunchUsesConfiguredWorkingDir` is still a single-case top-level test body instead of using the required `t.Run("Should...")` convention.
  - Root cause: the test was added without the file’s standard scenario wrapper.
  - Intended fix: wrap the body in a descriptive subtest and keep its current behavior assertions.
  - Resolution: wrapped the working-directory test body in a descriptive `t.Run("Should ...")` subtest.
