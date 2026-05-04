---
status: resolved
file: internal/daemon/boot_test.go
line: 560
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58go8M,comment:PRRC_kwDORy7nkc651UMt
---

# Issue 018: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Use `t.TempDir()` here instead of managing a temp root manually.**

This helper now bypasses the test harness' built-in temp-dir lifecycle and also ignores the cleanup error. `t.TempDir()` gives the same isolation with automatic cleanup and avoids the `_ = os.RemoveAll(...)` escape hatch.

<details>
<summary>♻️ Proposed fix</summary>

```diff
-	baseDir := os.TempDir()
-	if _, err := os.Stat("/tmp"); err == nil {
-		baseDir = "/tmp"
-	}
-	homeRoot, err := os.MkdirTemp(baseDir, "compozy-daemon-*")
-	if err != nil {
-		t.Fatalf("MkdirTemp() error = %v", err)
-	}
-	t.Cleanup(func() {
-		_ = os.RemoveAll(homeRoot)
-	})
+	homeRoot := t.TempDir()
```
</details>


As per coding guidelines, `**/*_test.go`: `Use t.TempDir() for filesystem isolation instead of manual temp directory management`; and `**/*.go`: `NEVER ignore errors with _ — every error must be handled or have a written justification`.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	homeRoot := t.TempDir()
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/boot_test.go` around lines 550 - 560, Replace the manual
temp-dir creation and cleanup with t.TempDir(): stop using
os.TempDir()/os.MkdirTemp and the homeRoot variable plus t.Cleanup +
os.RemoveAll; instead call homeRoot := t.TempDir() where the current MkdirTemp
result is used and remove the explicit t.Cleanup/_ = os.RemoveAll block so the
test harness manages cleanup and no errors are ignored.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:f507ecd8-2a5e-417f-9de9-d1c65fe7c2b9 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `mustHomePaths` manually creates and cleans up a temporary root, then silently ignores `os.RemoveAll` errors. The helper does need an explicitly short temp root for daemon socket-path limits, but it should not hide cleanup failures.
- Fix approach: keep the short-path temp-root strategy instead of blindly switching to `t.TempDir()`, and make the cleanup error explicit so the test harness reports teardown problems.
- Resolution: kept the short `/tmp` temp-root strategy, documented why it is required for daemon socket-length safety, and now report `os.RemoveAll` failures explicitly.
- Regression coverage: `go test ./internal/cli ./internal/core/run/journal ./internal/daemon` passed after the cleanup change.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.
