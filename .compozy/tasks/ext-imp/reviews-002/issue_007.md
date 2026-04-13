---
status: resolved
file: internal/core/run/executor/result_test.go
line: 281
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56V4Zl,comment:PRRC_kwDORy7nkc627G22
---

# Issue 007: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# First, let's find the test file and check its content
fd "result_test.go" -t f
```

Repository: compozy/compozy

Length of output: 100

---

🏁 Script executed:

```shell
# If found, let's examine the specific lines 245-281
if [ -f "internal/core/run/executor/result_test.go" ]; then
  wc -l "internal/core/run/executor/result_test.go"
  echo "---"
  sed -n '245,281p' "internal/core/run/executor/result_test.go"
fi
```

Repository: compozy/compozy

Length of output: 1313

---

🏁 Script executed:

```shell
# Also check the test structure and related code around it
if [ -f "internal/core/run/executor/result_test.go" ]; then
  sed -n '240,290p' "internal/core/run/executor/result_test.go"
fi
```

Repository: compozy/compozy

Length of output: 1598

---



</details>

**Refactor test to use table-driven subtests with `t.Run("Should...")` and `t.Parallel()`.**

The test currently uses an inline loop over formats without subtests. Per coding guidelines, this violates the mandatory requirement: "MUST use t.Run("Should...") pattern for ALL test cases." This prevents clear test failure identification and blocks `t.Parallel()` usage.

Each format case (JSON, RawJSON) should be wrapped in a named subtest with `t.Parallel()` for independent execution, and each subtest should have its own `t.TempDir()` call for proper isolation.

<details>
<summary>Refactor example</summary>

```diff
 func TestEmitExecutionResultKeepsWorkflowJSONModesQuietOnStdout(t *testing.T) {
-	runArtifacts := model.NewRunArtifacts(t.TempDir(), "workflow-run")
-	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
-		t.Fatalf("mkdir run dir: %v", err)
-	}
-
-	for _, format := range []model.OutputFormat{model.OutputFormatJSON, model.OutputFormatRawJSON} {
-		cfg := &config{
-			Mode:         model.ExecutionModePRDTasks,
-			IDE:          model.IDECodex,
-			Model:        "gpt-5.4",
-			OutputFormat: format,
-			RunArtifacts: runArtifacts,
-		}
-		result := executionResult{
-			RunID:        runArtifacts.RunID,
-			Mode:         string(cfg.Mode),
-			Status:       runStatusSucceeded,
-			IDE:          cfg.IDE,
-			Model:        cfg.Model,
-			OutputFormat: string(cfg.OutputFormat),
-			ArtifactsDir: runArtifacts.RunDir,
-			RunMetaPath:  runArtifacts.RunMetaPath,
-			ResultPath:   runArtifacts.ResultPath,
-		}
-
-		stdoutBytes := captureExecutionStdout(t, func() {
-			if err := emitExecutionResult(cfg, result); err != nil {
-				t.Fatalf("emitExecutionResult: %v", err)
-			}
-		})
-
-		if len(stdoutBytes) != 0 {
-			t.Fatalf("expected workflow %s mode to keep stdout quiet, got %q", format, string(stdoutBytes))
-		}
-	}
+	for _, format := range []model.OutputFormat{model.OutputFormatJSON, model.OutputFormatRawJSON} {
+		format := format
+		t.Run("ShouldKeepWorkflowStdoutQuiet_"+string(format), func(t *testing.T) {
+			t.Parallel()
+
+			runArtifacts := model.NewRunArtifacts(t.TempDir(), "workflow-run")
+			if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
+				t.Fatalf("mkdir run dir: %v", err)
+			}
+
+			cfg := &config{
+				Mode:         model.ExecutionModePRDTasks,
+				IDE:          model.IDECodex,
+				Model:        "gpt-5.4",
+				OutputFormat: format,
+				RunArtifacts: runArtifacts,
+			}
+			result := executionResult{
+				RunID:        runArtifacts.RunID,
+				Mode:         string(cfg.Mode),
+				Status:       runStatusSucceeded,
+				IDE:          cfg.IDE,
+				Model:        cfg.Model,
+				OutputFormat: string(cfg.OutputFormat),
+				ArtifactsDir: runArtifacts.RunDir,
+				RunMetaPath:  runArtifacts.RunMetaPath,
+				ResultPath:   runArtifacts.ResultPath,
+			}
+
+			stdoutBytes := captureExecutionStdout(t, func() {
+				if err := emitExecutionResult(cfg, result); err != nil {
+					t.Fatalf("emitExecutionResult: %v", err)
+				}
+			})
+
+			if len(stdoutBytes) != 0 {
+				t.Fatalf("expected workflow %s mode to keep stdout quiet, got %q", format, string(stdoutBytes))
+			}
+		})
+	}
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/executor/result_test.go` around lines 245 - 281, Refactor
TestEmitExecutionResultKeepsWorkflowJSONModesQuietOnStdout to use table-driven
subtests: replace the inline loop over model.OutputFormat with a slice of cases
and for each case call t.Run("Should keep stdout quiet for <format>") and inside
the subtest call t.Parallel(), create a fresh runArtifacts via t.TempDir(),
build cfg and executionResult as before, capture stdout with
captureExecutionStdout, and assert zero output; keep references to
emitExecutionResult, captureExecutionStdout, config, executionResult and
runArtifacts so the same logic is used but isolated per subtest.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:65ad56ad-6266-4a6e-86ad-c70cd9fe2d62 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Notes:
  - The current test already covers both workflow JSON formats with deterministic assertions, and stdout capture is serialized through a package-level mutex.
  - Converting the loop into parallel subtests would be a stylistic refactor, not a correctness fix; it would not expand the behavioral surface under test.
  - No code change is planned for this issue.
