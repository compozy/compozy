---
status: valid
file: internal/daemon/boot_test.go
line: 150
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57_RAM,comment:PRRC_kwDORy7nkc65JPyX
---

# Issue 007: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Do not discard `Host.Close` errors in cleanup.**

Line 149 currently ignores the cleanup error, which can hide teardown failures and leak state between tests.  
As per coding guidelines, "NEVER ignore errors with `_` — every error must be handled or have a written justification".  


<details>
<summary>Proposed fix</summary>

```diff
-	defer func() {
-		_ = result.Host.Close(context.Background())
-	}()
+	t.Cleanup(func() {
+		if closeErr := result.Host.Close(context.Background()); closeErr != nil {
+			t.Errorf("Host.Close() error = %v", closeErr)
+		}
+	})
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	t.Cleanup(func() {
		if closeErr := result.Host.Close(context.Background()); closeErr != nil {
			t.Errorf("Host.Close() error = %v", closeErr)
		}
	})
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/boot_test.go` around lines 148 - 150, The test cleanup
currently discards the error returned by
result.Host.Close(context.Background()); update the deferred function to capture
that error and handle it (e.g., call t.Fatalf/t.Errorf or t.Helper plus
require.NoError) instead of using the blank identifier; specifically change the
defer that wraps result.Host.Close to check its returned error and fail or log
the test accordingly so Host.Close errors are not ignored.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:112af2e5-813e-451e-9276-366cfc5878ac -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `internal/daemon/boot_test.go` still has multiple test cleanups that call `result.Host.Close(context.Background())` and discard the returned error with `_ = ...`.
- Why this matters: teardown failures would be silent, which can hide leaked runtime state and make later daemon tests harder to diagnose.
- Fix approach: replace the ignored deferred cleanup with test cleanup that checks the returned error and reports it through the active `testing.T`.
