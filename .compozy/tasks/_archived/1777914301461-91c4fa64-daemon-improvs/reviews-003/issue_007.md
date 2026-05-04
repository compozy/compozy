---
status: resolved
file: internal/logger/logger_test.go
line: 177
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58jijA,comment:PRRC_kwDORy7nkc655NSY
---

# Issue 007: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
set -euo pipefail

# Verify top-level tests vs required subtest style in this file
rg -n '^func Test' internal/logger/logger_test.go
rg -n 't\.Run\("Should' internal/logger/logger_test.go || true
```

Repository: compozy/compozy

Length of output: 491

---



</details>

**Refactor tests to use `t.Run("Should...")` subtests with table-driven pattern for all test cases.**

All 6 tests in this file are top-level functions instead of subtests within a table-driven structure. Per coding guidelines, this violates the required pattern: "MUST use `t.Run("Should...")` pattern for ALL test cases" and "Use table-driven tests with subtests (`t.Run`) as the default pattern."

Additionally, line 118 ignores the error from `writer.Close()` with `_ =` and provides no written justification. Per guidelines: "NEVER ignore errors with `_` — every error must be handled or have a written justification."

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/logger/logger_test.go` around lines 12 - 177, Several top-level
tests (TestInstallDaemonLoggerForegroundMirrorsToStderrAndFile,
TestInstallDaemonLoggerDetachedWritesOnlyFile,
TestOpenRotatingFileRotatesAtConfiguredSize,
TestOpenRotatingFileKeepsWritingAfterRotationFailure,
TestValidateDaemonFilePathRejectsEmptyPath,
TestNormalizeFilePathCleansRelativeSegments) must be refactored into a
table-driven pattern using t.Run("Should...") subtests; convert these cases into
a single Test... function that iterates a slice of test cases and invokes t.Run
with descriptive "Should..." names for each scenario, keeping existing
assertions and reusing helpers like InstallDaemonLogger, openRotatingFile,
ValidateDaemonFilePath, and normalizeFilePath to locate logic. Also fix the
ignored error in TestOpenRotatingFileKeepsWritingAfterRotationFailure by
checking the return value from writer.Close() (the writer variable from
openRotatingFile) and handling/failing the test on error instead of using `_ =
writer.Close()`, or add an explicit comment justifying why it can be ignored if
you genuinely choose that route.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:1ee37aad-e1fe-4d03-9d0d-0fb21121abc4 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `INVALID`
- Reasoning: This comment duplicates the concrete close-error finding from issue `005` and overreaches by demanding a single table-driven refactor across six unrelated tests that cover different APIs and behaviors.
- Root cause: The claimed blanket rule does not match the documented repository guidance, which prefers subtests by default but does not require collapsing distinct single-scenario tests into one shared table.
- Resolution plan: No direct refactor for this issue. Address the real close-error defect through issue `005` and keep the remaining tests separated by behavior.

## Resolution

- Closed as `invalid`. Issue `005` covered the real close-error defect, and the larger table-driven rewrite requested here was not required to fix a correctness or reliability problem.

## Verification

- Confirmed against the current file state and completed a fresh `make verify` pass after the in-scope fixes for the valid issues.
