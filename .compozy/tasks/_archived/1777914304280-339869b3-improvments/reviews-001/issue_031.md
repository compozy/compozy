---
status: resolved
file: internal/daemon/run_manager_test.go
line: 204
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:5815061f3022
review_hash: 5815061f3022
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 031: Please wrap these added cases in t.Run("Should...") subtests.
## Review Comment

They are currently added as standalone top-level tests, which breaks the repo’s required test-case pattern and makes the new coverage less uniform with the surrounding suite.

As per coding guidelines, "MUST use t.Run("Should...") pattern for ALL test cases".

Also applies to: 636-676

## Triage

- Decision: `valid`
- Notes: Confirmed newly added run manager behavioral tests were standalone bodies rather than `t.Run("Should ...")` cases. Wrapped the affected tests in descriptive subtests.
