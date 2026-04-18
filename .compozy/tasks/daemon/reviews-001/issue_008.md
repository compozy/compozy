---
status: resolved
file: internal/api/core/handlers_error_paths_test.go
line: 17
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mmx,comment:PRRC_kwDORy7nkc65HKX7
---

# Issue 008: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# First, locate and examine the test file
find . -name "handlers_error_paths_test.go" -type f
```

Repository: compozy/compozy

Length of output: 107

---

🏁 Script executed:

```shell
# Check if the file exists and read the test content
cat -n internal/api/core/handlers_error_paths_test.go | head -50
```

Repository: compozy/compozy

Length of output: 1438

---

🏁 Script executed:

```shell
# Search for other gin.SetMode calls in the same package
rg "gin\.SetMode" internal/api/core/ -t go
```

Repository: compozy/compozy

Length of output: 540

---

🏁 Script executed:

```shell
# Check the package structure and other test files
ls -la internal/api/core/*_test.go | head -20
```

Repository: compozy/compozy

Length of output: 604

---

🏁 Script executed:

```shell
# Check if any test files restore gin mode after SetMode
rg -A 5 "gin\.SetMode\(gin\.TestMode\)" internal/api/core/ -t go
```

Repository: compozy/compozy

Length of output: 2914

---

🏁 Script executed:

```shell
# Check if any test files use t.Cleanup with gin.SetMode
rg "t\.Cleanup" internal/api/core/ -t go -B 3 -A 3
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Verify Gin library behavior - check how gin.SetMode affects global state
# Let's look at one of the other test files to see if they have the same issue
cat -n internal/api/core/handlers_test.go | head -30
```

Repository: compozy/compozy

Length of output: 806

---

🏁 Script executed:

```shell
# Check the handlers_service_errors_test.go file as well
cat -n internal/api/core/handlers_service_errors_test.go | head -30
```

Repository: compozy/compozy

Length of output: 797

---

🏁 Script executed:

```shell
# Check if TestSharedHandlersValidationAndServiceErrors uses t.Parallel
grep -A 20 "func TestSharedHandlersValidationAndServiceErrors" internal/api/core/handlers_error_paths_test.go | head -25
```

Repository: compozy/compozy

Length of output: 576

---

🏁 Script executed:

```shell
# Check if there's any gin.Mode() restoration pattern in the codebase
rg "gin\.Mode\(\)" internal/api/core/ -t go
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Verify Gin documentation/behavior by checking gin package usage patterns
rg "gin\.SetMode" . -t go | wc -l
```

Repository: compozy/compozy

Length of output: 61

---

🏁 Script executed:

```shell
# Check the full test function to see if it has t.Run and t.Parallel patterns
cat -n internal/api/core/handlers_error_paths_test.go | sed -n '15,150p'
```

Repository: compozy/compozy

Length of output: 4062

---

🏁 Script executed:

```shell
# Look for t.Run and t.Parallel usage in the test
grep -n "t\.Run\|t\.Parallel" internal/api/core/handlers_error_paths_test.go
```

Repository: compozy/compozy

Length of output: 100

---

🏁 Script executed:

```shell
# Check if tests using t.Parallel also call gin.SetMode without cleanup
cat -n internal/api/core/handlers_test.go | sed -n '19,30p'
```

Repository: compozy/compozy

Length of output: 420

---

🏁 Script executed:

```shell
# Verify the complete picture - see the test structure that uses t.Parallel
cat -n internal/api/core/handlers_service_errors_test.go | sed -n '20,45p'
```

Repository: compozy/compozy

Length of output: 766

---



</details>

**Restore Gin mode after this test.**

`gin.SetMode` mutates process-global state without cleanup. Restore `gin.Mode()` via `t.Cleanup()` to prevent state leakage into subsequent tests.

<details>
<summary>Suggested fix</summary>

```diff
 func TestSharedHandlersValidationAndServiceErrors(t *testing.T) {
-	gin.SetMode(gin.TestMode)
+	previousMode := gin.Mode()
+	gin.SetMode(gin.TestMode)
+	t.Cleanup(func() { gin.SetMode(previousMode) })
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestSharedHandlersValidationAndServiceErrors(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/handlers_error_paths_test.go` around lines 15 - 17, The
test TestSharedHandlersValidationAndServiceErrors calls gin.SetMode which
mutates global Gin state; add a t.Cleanup that captures the current mode via old
:= gin.Mode() before calling gin.SetMode(gin.TestMode) and restore it in cleanup
with gin.SetMode(old) so subsequent tests are not affected (reference
gin.SetMode, gin.Mode, and TestSharedHandlersValidationAndServiceErrors).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:b0da8a0d-7451-4c83-b7b6-de09a795b9a0 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: `TestSharedHandlersValidationAndServiceErrors` mutates Gin’s process-global mode and never restores the previous value.
- Fix plan: capture the previous mode before switching to `gin.TestMode` and restore it with `t.Cleanup`.
- Resolution: Implemented and verified with `make verify`.
