---
status: resolved
file: internal/cli/form_test.go
line: 20
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:72a554f21af3
review_hash: 72a554f21af3
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 015: Wrap these added cases in t.Run("Should...") subtests.
## Review Comment

The new cases are introduced as standalone top-level tests, so they drift from the repo’s required test-case structure and make reporting less consistent than the rest of the suite.

As per coding guidelines, "MUST use t.Run("Should...") pattern for ALL test cases".

Also applies to: 77-145, 328-367

## Triage

- Decision: `VALID`
- Notes: The added task-run form cases were top-level assertions instead of `Should...` subtests. Wrapped the affected cases, including the runtime preseed case, in named subtests while preserving behavior.
