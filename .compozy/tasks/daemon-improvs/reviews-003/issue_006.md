---
status: resolved
file: internal/logger/logger_test.go
line: 136
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58jijY,comment:PRRC_kwDORy7nkc655NS7
---

# Issue 006: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
set -euo pipefail

# Find weak non-nil-only error checks in this test file
rg -nP 'err\s*==\s*nil' internal/logger/logger_test.go

# Inspect logger package error definitions/wrapping targets to use errors.Is/errors.As
rg -n 'var\s+Err|errors\.New|fmt\.Errorf\(' internal/logger/logger.go
```

Repository: compozy/compozy

Length of output: 1513

---

🏁 Script executed:

```shell
# Read the test file around the flagged lines to understand test context
sed -n '130,140p' internal/logger/logger_test.go
sed -n '158,168p' internal/logger/logger_test.go

# Check if ValidateDaemonFilePath and writer.Write are expected to return specific errors
rg -B 5 -A 5 'func.*ValidateDaemonFilePath' internal/logger/logger.go
rg -B 5 -A 5 'type.*[Rr]otating' internal/logger/logger.go
```

Repository: compozy/compozy

Length of output: 1343

---

🏁 Script executed:

```shell
# Check normalizeFilePath implementation to see what error it returns
rg -A 10 'func.*normalizeFilePath' internal/logger/logger.go

# Check rotatingFile.Write to see what specific error it returns when rotation fails
rg -B 2 -A 15 'func.*\(r \*rotatingFile\) Write' internal/logger/logger.go
```

Repository: compozy/compozy

Length of output: 723

---

🏁 Script executed:

```shell
# Check if logger package exports sentinel errors that tests can use
rg -n 'var\s+\w+\s+error\s*=' internal/logger/logger.go

# Check what testing assertion libraries are imported in the test file
head -20 internal/logger/logger_test.go | grep -i 'import\|assert\|require'

# Check rotateIfNeededLocked to understand what error it returns
rg -B 2 -A 20 'func.*\(r \*rotatingFile\) rotateIfNeededLocked' internal/logger/logger.go
```

Repository: compozy/compozy

Length of output: 630

---

🏁 Script executed:

```shell
# Check if any custom error types are defined in logger package
rg -n 'type\s+\w+\s+(struct|interface)\s*{' internal/logger/logger.go | head -10

# Check complete test file structure to see if errors are used with ErrorContains or similar
rg -n 'ErrorContains|ErrorAs|assert|require' internal/logger/logger_test.go

# Verify what testing assertion library is used (if any)
head -30 internal/logger/logger_test.go
```

Repository: compozy/compozy

Length of output: 837

---

🏁 Script executed:

```shell
# Check if any sentinel errors are exported from logger package
rg -n 'var\s+Err' internal/logger/logger.go

# Get complete view of errors defined in logger.go
rg -n 'errors\.New|fmt\.Errorf' internal/logger/logger.go | head -20

# Check what the Write() call at line 134 is actually testing - full context
sed -n '125,145p' internal/logger/logger_test.go
```

Repository: compozy/compozy

Length of output: 2079

---

🏁 Script executed:

```shell
# Check if logger package exports any error variables or custom error types
grep -n 'var\|type' internal/logger/logger.go | grep -i 'err\|error'

# Full view of rotateLogFiles to see what errors it returns
rg -B 2 -A 20 'func rotateLogFiles' internal/logger/logger.go

# Check full test for line 134 - what is 'writer' and what test is this part of
sed -n '100,145p' internal/logger/logger_test.go
```

Repository: compozy/compozy

Length of output: 2130

---

🏁 Script executed:

```shell
# Check what error types are wrapped in rotateLogFiles chain
rg -B 5 -A 10 'func renameLogFile' internal/logger/logger.go

# Verify if os package errors can be matched with errors.As
rg -n 'os\.' internal/logger/logger.go | head -10
```

Repository: compozy/compozy

Length of output: 965

---

🏁 Script executed:

```shell
# Verify what the rotateIfNeededLocked returns when it fails - does it wrap the error from rotateLogFiles?
sed -n '215,245p' internal/logger/logger.go

# Check imports in logger.go to see if custom error types are used anywhere
head -15 internal/logger/logger.go
```

Repository: compozy/compozy

Length of output: 1078

---



</details>

**Use `errors.As()` to validate wrapped error types in failed rotation scenario; exported error sentinel required for path validation check.**

Line 134-136: The `Write()` call during rotation failure returns a wrapped OS error from file operations. Replace the nil check with `errors.As()` to validate it's a file operation error (e.g., `os.PathError`), not an unrelated error.

Line 162-164: The `ValidateDaemonFilePath()` error is unwrapped (`errors.New()`). To use `errors.Is()` per coding guidelines, the logger package must first export a sentinel error variable (e.g., `var ErrInvalidPath = errors.New(...)`), then update the test to use it. Alternatively, validate the error message directly.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/logger/logger_test.go` around lines 134 - 136, The test currently
only checks for a non-nil error from writer.Write during a rotation failure;
change that assertion to use errors.As to confirm the returned (possibly
wrapped) error is a file operation error such as *os.PathError (i.e., replace
the nil-check with an errors.As check against os.PathError when calling
writer.Write in the rotation test). Also make the package export a sentinel
error (e.g., var ErrInvalidPath = errors.New("invalid daemon path")) used by
ValidateDaemonFilePath, then update the test to assert errors.Is(err,
logger.ErrInvalidPath) (or alternatively compare the error string) so the
ValidateDaemonFilePath failure can be detected via errors.Is rather than an
unwrapped errors.New.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:1ee37aad-e1fe-4d03-9d0d-0fb21121abc4 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Reasoning: The rotation-failure test only asserts that an error is non-nil, so it would pass for unrelated failures. The empty-path validation test can also assert the stable expected error rather than just "non-nil".
- Root cause: The assertions are too weak to prove the intended wrapped filesystem error and the specific empty-path validation failure.
- Resolution plan: Use `errors.As` to confirm the wrapped `*os.PathError` from the forced rotation failure, and tighten the empty-path assertion using the current stable error text without widening production API surface outside the scoped files.

## Resolution

- Strengthened `TestOpenRotatingFileKeepsWritingAfterRotationFailure` to assert the forced rotation failure wraps `*os.PathError`.
- Tightened `TestValidateDaemonFilePathRejectsEmptyPath` to assert the exact expected validation error text, keeping the fix inside the scoped test file instead of widening the logger API.

## Verification

- `go test ./internal/api/core ./internal/daemon ./internal/logger -count=1`
- `make verify`
