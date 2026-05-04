---
status: resolved
file: internal/api/core/internal_helpers_test.go
line: 16
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4134921970,nitpick_hash:1b235ccef0d1
review_hash: 1b235ccef0d1
source_review_id: "4134921970"
source_review_submitted_at: "2026-04-18T18:54:28Z"
---

# Issue 010: Comprehensive but monolithic test for Problem and error helpers.
## Review Comment

This test covers many aspects of the error handling system:
- Problem.Error() formatting and fallbacks
- errors.Is() wrapping behavior
- Nil-pointer safety
- Status/code/message mapping for various error types
- Helper constructors (invalidJSONProblem, serviceUnavailableProblem)
- Schema error details extraction
- Context cancellation detection

Consider splitting into focused subtests using `t.Run` to improve test failure diagnostics and align with the table-driven test pattern guideline.

As per coding guidelines: "Table-driven tests with subtests (`t.Run`) as the default pattern".

## Triage

- Decision: `VALID`
- Root cause: `TestProblemAndHelperFunctions` packs several unrelated behaviors into one long test body, which makes failures harder to localize and diverges from the project’s preferred subtest style.
- Fix plan: split the coverage into focused `t.Run` groups while preserving the existing assertions and behavior coverage.
- Resolution: Implemented and verified with `make verify`.
