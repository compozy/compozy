---
status: resolved
file: internal/core/run/exec/exec_test.go
line: 205
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56QZsW,comment:PRRC_kwDORy7nkc620G-1
---

# Issue 002: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# Check the test file exists and examine the actual structure
cd internal/core/run/exec || exit 1
wc -l exec_test.go
```

Repository: compozy/compozy

Length of output: 75

---

🏁 Script executed:

```shell
# Get specific sections to verify test patterns
cd internal/core/run/exec || exit 1
sed -n '20,205p' exec_test.go | head -100
```

Repository: compozy/compozy

Length of output: 3021

---

🏁 Script executed:

```shell
# Check for test helper functions mentioned in scratchpad
cd internal/core/run/exec || exit 1
grep -n "func.*For.*Test\|func.*Stub\|func.*Content.*Block" exec_test.go | head -20
```

Repository: compozy/compozy

Length of output: 254

---

🏁 Script executed:

```shell
# Search for any panic usage in test file
cd internal/core/run/exec || exit 1
rg "panic\(" exec_test.go -n
```

Repository: compozy/compozy

Length of output: 75

---

🏁 Script executed:

```shell
# Examine the helper functions in detail
cd internal/core/run/exec || exit 1
sed -n '437,471p' exec_test.go
```

Repository: compozy/compozy

Length of output: 993

---

🏁 Script executed:

```shell
# Check if test helpers should use t.Helper() per guidelines
cd internal/core/run/exec || exit 1
sed -n '437,475p' exec_test.go
```

Repository: compozy/compozy

Length of output: 993

---

🏁 Script executed:

```shell
# Check the full list of test functions in the file
cd internal/core/run/exec || exit 1
rg "^func Test" exec_test.go -o
```

Repository: compozy/compozy

Length of output: 188

---

🏁 Script executed:

```shell
# Look at the error assertion patterns more carefully across all tests
cd internal/core/run/exec || exit 1
rg "if err ==" exec_test.go -A 1 -B 1 | head -60
```

Repository: compozy/compozy

Length of output: 732

---

🏁 Script executed:

```shell
# Get full test function names to see all tests
cd internal/core/run/exec || exit 1
rg "^func Test\w+" exec_test.go -o
```

Repository: compozy/compozy

Length of output: 781

---

🏁 Script executed:

```shell
# Check if there are any table-driven tests elsewhere in the file for reference
cd internal/core/run/exec || exit 1
rg "t\.Run\(" exec_test.go | head -20
```

Repository: compozy/compozy

Length of output: 41

---



</details>

**Restructure tests to table-driven `t.Run("Should...")` pattern with specific error assertions, and fix `panic` in helper.**

Tests in lines 20-205 are not table-driven and lack proper error validation:
- `TestExecutePreparedPromptValidatesInputs` has two related validation cases that should be combined into one table-driven test with subtests for missing config and empty prompt
- Multiple tests use weak assertions like `if err == nil { t.Fatal("expected X error") }` without validating error type or message
- `preparedPromptTextContentBlock` (line 459) uses `panic(err)` instead of following the established pattern in other helpers (`t.Helper()` + `t.Fatalf`)

Per coding guidelines, all test cases must use `t.Run("Should...")` subtests with specific error assertions using `ErrorContains` or `ErrorAs`, and all test helpers must be marked with `t.Helper()`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/exec/exec_test.go` around lines 20 - 205, Combine the
related validation cases in TestExecutePreparedPromptValidatesInputs into a
table-driven set of subtests using t.Run("Should ...") (e.g., "Should error when
config is nil", "Should error when prompt is empty") that call
ExecutePreparedPrompt and assert specific errors using ErrorContains/ErrorAs or
errors.Is/strings.Contains rather than only checking err == nil; update other
tests that currently do weak assertions to similarly assert the expected error
type/message. Also fix the helper preparedPromptTextContentBlock to call
t.Helper() and replace any panic(err) with t.Fatalf to surface failures via the
testing.T; use the function names ExecutePreparedPrompt,
TestExecutePreparedPromptValidatesInputs, and preparedPromptTextContentBlock to
locate the changes. Ensure each subtest uses t.Parallel appropriately and
preserves existing setup/cleanup semantics.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:3bf551e7-5f49-4b28-8f83-cabd39a9c4c5 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes:
  - `TestExecutePreparedPromptValidatesInputs` currently checks only `err == nil`, so it does not pin the exact validation failures for the nil-config and empty-prompt paths.
  - `preparedPromptTextContentBlock` currently panics on JSON marshal failure and is not marked as a test helper, which weakens failure attribution.
  - Root cause: the review-targeted tests do not assert the specific error contract of `ExecutePreparedPrompt`, and the helper bypasses `testing.T` on failure.
  - Fix approach: convert the validation coverage to `t.Run("Should ...")` subtests with specific error assertions and move the helper failure path onto `testing.T`.
  - Resolved in `internal/core/run/exec/exec_test.go` with table-driven validation subtests, stronger runtime-availability assertions, and a helper-aware `preparedPromptTextContentBlock`.
  - Verified by `go test ./internal/core/run/exec -count=1` and the final `make verify` pass.
