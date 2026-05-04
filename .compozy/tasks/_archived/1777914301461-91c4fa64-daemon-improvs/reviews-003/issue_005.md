---
status: resolved
file: internal/logger/logger_test.go
line: 120
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58jijF,comment:PRRC_kwDORy7nkc655NSd
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
set -euo pipefail

# Verify ignored-close and multiple close call sites in the same test
rg -n -C2 '_\s*=\s*writer\.Close\(\)|writer\.Close\(\)' internal/logger/logger_test.go
```

Repository: compozy/compozy

Length of output: 378

---

🏁 Script executed:

```shell
head -n 160 internal/logger/logger_test.go | tail -n 50
```

Repository: compozy/compozy

Length of output: 1417

---

🏁 Script executed:

```shell
sed -n '110,155p' internal/logger/logger_test.go | cat -n
```

Repository: compozy/compozy

Length of output: 1657

---

🏁 Script executed:

```shell
rg -n "func.*rotatingFile.*Close" internal/logger/logger.go -A 15
```

Repository: compozy/compozy

Length of output: 481

---

🏁 Script executed:

```shell
sed -n '191,215p' internal/logger/logger.go | cat -n
```

Repository: compozy/compozy

Length of output: 780

---



</details>

**Remove the ignored close error and redundant defer in `TestOpenRotatingFileKeepsWritingAfterRotationFailure`.**

Line 119 ignores a close error (`_ = writer.Close()`), violating the coding guideline that errors must never be ignored without justification. Additionally, the defer is redundant since an explicit `writer.Close()` with proper error handling occurs at line 149, making the defer close attempt unnecessary.

<details>
<summary>🔧 Suggested fix</summary>

```diff
 func TestOpenRotatingFileKeepsWritingAfterRotationFailure(t *testing.T) {
 	logPath := filepath.Join(t.TempDir(), "daemon.log")
 	writer, err := openRotatingFile(rotatingFileConfig{
 		path:             logPath,
 		maxFileSizeBytes: 16,
 		maxRetainedFiles: 2,
 		filePerm:         defaultDaemonLogPerm,
 	})
 	if err != nil {
 		t.Fatalf("openRotatingFile() error = %v", err)
 	}
-	defer func() {
-		_ = writer.Close()
-	}()
 
 	if _, err := writer.Write([]byte("seed-entry\n")); err != nil {
 		t.Fatalf("Write(seed) error = %v", err)
```

</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion

```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/logger/logger_test.go` around lines 118 - 120, In
TestOpenRotatingFileKeepsWritingAfterRotationFailure remove the redundant defer
that calls _ = writer.Close() and do not ignore the close error; delete that
defer block and rely on the existing explicit writer.Close() at the end of the
test (or convert it to t.Cleanup) and assert/check its error (e.g.,
require.NoError/if err != nil t.Fatalf) so the writer variable is closed exactly
once with proper error handling.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:1ee37aad-e1fe-4d03-9d0d-0fb21121abc4 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Reasoning: `TestOpenRotatingFileKeepsWritingAfterRotationFailure` closes the writer twice and ignores the first close error via `_ = writer.Close()`.
- Root cause: The test mixes a blind deferred close with a later explicit close, so close failures can be lost and cleanup order becomes ambiguous.
- Resolution plan: Replace the ignored defer with cleanup that closes at most once and surfaces unexpected close failures.

## Resolution

- Replaced the ignored deferred close in `internal/logger/logger_test.go` with a guarded `t.Cleanup` that reports cleanup failures and avoids double-closing after the explicit successful close.

## Verification

- `go test ./internal/api/core ./internal/daemon ./internal/logger -count=1`
- `make verify`
