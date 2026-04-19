---
status: resolved
file: internal/daemon/boot_test.go
line: 163
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57_RAL,comment:PRRC_kwDORy7nkc65JPyW
---

# Issue 002: _🛠️ Refactor suggestion_ | _🟠 Major_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_

**Wrap this case in a `t.Run("Should...")` subtest.**

Line 132 introduces a standalone case, but test policy requires the subtest pattern for all cases in this suite.  
As per coding guidelines, "MUST use t.Run("Should...") pattern for ALL test cases".  


<details>
<summary>Proposed refactor</summary>

```diff
 func TestStartDefaultsHTTPPortWhenUnset(t *testing.T) {
 	t.Parallel()
-
-	paths := mustHomePaths(t)
-	result, err := Start(context.Background(), StartOptions{
-		HomePaths: paths,
-		PID:       5151,
-		Version:   "default-http-port",
-		Now: func() time.Time {
-			return time.Unix(50, 0).UTC()
-		},
-		ProcessAlive: func(pid int) bool { return pid == 5151 },
-	})
-	if err != nil {
-		t.Fatalf("Start() error = %v", err)
-	}
-	defer func() {
-		_ = result.Host.Close(context.Background())
-	}()
-
-	if result.Info.HTTPPort != DefaultHTTPPort {
-		t.Fatalf("Info.HTTPPort = %d, want %d", result.Info.HTTPPort, DefaultHTTPPort)
-	}
-
-	currentInfo, err := ReadInfo(paths.InfoPath)
-	if err != nil {
-		t.Fatalf("ReadInfo() error = %v", err)
-	}
-	if currentInfo.HTTPPort != DefaultHTTPPort {
-		t.Fatalf("currentInfo.HTTPPort = %d, want %d", currentInfo.HTTPPort, DefaultHTTPPort)
-	}
+	t.Run("Should default HTTP port when unset", func(t *testing.T) {
+		t.Parallel()
+
+		paths := mustHomePaths(t)
+		result, err := Start(context.Background(), StartOptions{
+			HomePaths: paths,
+			PID:       5151,
+			Version:   "default-http-port",
+			Now: func() time.Time {
+				return time.Unix(50, 0).UTC()
+			},
+			ProcessAlive: func(pid int) bool { return pid == 5151 },
+		})
+		if err != nil {
+			t.Fatalf("Start() error = %v", err)
+		}
+		defer func() {
+			_ = result.Host.Close(context.Background())
+		}()
+
+		if result.Info.HTTPPort != DefaultHTTPPort {
+			t.Fatalf("Info.HTTPPort = %d, want %d", result.Info.HTTPPort, DefaultHTTPPort)
+		}
+
+		currentInfo, err := ReadInfo(paths.InfoPath)
+		if err != nil {
+			t.Fatalf("ReadInfo() error = %v", err)
+		}
+		if currentInfo.HTTPPort != DefaultHTTPPort {
+			t.Fatalf("currentInfo.HTTPPort = %d, want %d", currentInfo.HTTPPort, DefaultHTTPPort)
+		}
+	})
 }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestStartDefaultsHTTPPortWhenUnset(t *testing.T) {
	t.Parallel()
	t.Run("Should default HTTP port when unset", func(t *testing.T) {
		t.Parallel()

		paths := mustHomePaths(t)
		result, err := Start(context.Background(), StartOptions{
			HomePaths: paths,
			PID:       5151,
			Version:   "default-http-port",
			Now: func() time.Time {
				return time.Unix(50, 0).UTC()
			},
			ProcessAlive: func(pid int) bool { return pid == 5151 },
		})
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		defer func() {
			_ = result.Host.Close(context.Background())
		}()

		if result.Info.HTTPPort != DefaultHTTPPort {
			t.Fatalf("Info.HTTPPort = %d, want %d", result.Info.HTTPPort, DefaultHTTPPort)
		}

		currentInfo, err := ReadInfo(paths.InfoPath)
		if err != nil {
			t.Fatalf("ReadInfo() error = %v", err)
		}
		if currentInfo.HTTPPort != DefaultHTTPPort {
			t.Fatalf("currentInfo.HTTPPort = %d, want %d", currentInfo.HTTPPort, DefaultHTTPPort)
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

In `@internal/daemon/boot_test.go` around lines 132 - 163, Wrap the existing
TestStartDefaultsHTTPPortWhenUnset test body in a t.Run subtest named with the
"Should..." pattern (e.g., t.Run("Should default HTTP port when unset", func(t
*testing.T) { ... })), moving t.Parallel() into the subtest so the test still
runs in parallel; keep the calls to mustHomePaths, Start, DefaultHTTPPort,
ReadInfo and the existing assertions and defer inside that subtest body
unchanged.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:112af2e5-813e-451e-9276-366cfc5878ac -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Reasoning: the flagged test is a single-scenario top-level Go test, and the current `internal/daemon/boot_test.go` file uses that same flat pattern throughout the suite. There are no existing `t.Run(...)` subtests in this file, and a repository search only finds the quoted `t.Run("Should...")` requirement inside generated review artifacts rather than in `AGENTS.md`, `CLAUDE.md`, or any checked-in test guidance.
- Root cause analysis: this review comment is a style preference, not a correctness, safety, or maintainability defect in the current test. Wrapping `TestStartDefaultsHTTPPortWhenUnset` in a one-case `t.Run(...)` block would not change behavior, improve coverage, or align it with an actual file-local convention.
- Resolution: left `internal/daemon/boot_test.go` unchanged and closed this issue as a stale/unsupported style suggestion against the current branch state.
- Verification: `go test ./internal/daemon -run 'TestStartDefaultsHTTPPortWhenUnset|TestStartRemovesStaleArtifactsAndMarksReady' -count=1` passed. `make verify` also passed with formatting and lint clean, `2410` tests executed, `1` intentionally skipped helper-process test, and a successful `go build ./cmd/compozy`.
