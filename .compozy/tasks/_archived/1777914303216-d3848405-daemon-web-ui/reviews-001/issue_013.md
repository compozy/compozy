---
status: resolved
file: internal/daemon/host_runtime_test.go
line: 28
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqXe,comment:PRRC_kwDORy7nkc651WHU
---

# Issue 013: _🛠️ Refactor suggestion_ | _🟠 Major_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
cat -n internal/daemon/host_runtime_test.go | head -50
```

Repository: compozy/compozy

Length of output: 1643

---

🏁 Script executed:

```shell
cat -n internal/daemon/host_runtime_test.go | sed -n '20,100p'
```

Repository: compozy/compozy

Length of output: 3162

---



</details>

**Use `t.TempDir()` instead of manual temp directory management.**

The test currently creates a temporary directory with `os.MkdirTemp("/tmp", ...)` and manually cleans it up. The coding guidelines require using `t.TempDir()` for filesystem isolation. Replace lines 21-27 with `homeDir := t.TempDir()`.

<details>
<summary>Suggested change</summary>

```diff
-	homeDir, err := os.MkdirTemp("/tmp", "daemon-host-*")
-	if err != nil {
-		t.Fatalf("MkdirTemp(/tmp) error = %v", err)
-	}
-	t.Cleanup(func() {
-		_ = os.RemoveAll(homeDir)
-	})
+	homeDir := t.TempDir()
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/host_runtime_test.go` around lines 21 - 28, Replace the
manual temp dir creation and cleanup (the os.MkdirTemp call, error check, and
t.Cleanup removal) with t.TempDir(): assign homeDir := t.TempDir() and keep the
existing t.Setenv("HOME", homeDir); remove the os.MkdirTemp, error handling, and
t.Cleanup related to homeDir so the test uses t.TempDir() for automatic cleanup.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5b81ebf2-33d3-49d0-b9c4-2c97e797915b -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The test uses manual temp-directory allocation and cleanup even though the repository standard requires `t.TempDir()` for isolation.
  - Root cause: older temp-dir setup was carried forward unchanged.
  - Intended fix: replace the manual temp-dir lifecycle with `t.TempDir()`.

## Resolution

- Replaced the manual temporary-directory lifecycle in `host_runtime_test.go` with `t.TempDir()`.
- Verified with `make verify`.
