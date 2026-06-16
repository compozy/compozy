---
provider: coderabbit
pr: "201"
round: 2
round_created_at: 2026-06-15T19:35:34.422493Z
status: resolved
file: internal/daemon/run_snapshot_test.go
line: 257
severity: major
author: coderabbitai[bot]
provider_ref: review:4500514537,nitpick_hash:62dbbea950aa
review_hash: 62dbbea950aa
source_review_id: "4500514537"
source_review_submitted_at: "2026-06-15T19:35:05Z"
---

# Issue 003: Align new tests with the required t.Run("Should...") structure.
## Review Comment

The new test coverage here still mixes non-compliant naming (`t.Run(tc.name, ...)` with values like `"pausing"`) and a standalone top-level case without a `t.Run("Should...")` wrapper. Please wrap each test case in `Should...` subtests and update table case names accordingly.

As per coding guidelines, `**/*_test.go` must use `t.Run("Should...")` for all test cases and use table-driven tests with subtests as the default pattern.

Also applies to: 345-372

<!-- cr-comment:v1:d25ba8ae039f070bb24d0be4 -->

_Source: Coding guidelines_

## Triage

- Decision: `valid`
- Root cause: The new daemon snapshot regressions include table case names such as `"pausing"`/`"paused"`/`"resumed"` that are passed directly to `t.Run`, and `TestRunSnapshotBuilderInfersSparseQueuedTaskNumber` performs assertions at the top level instead of inside a `t.Run("Should...")` subtest.
- Fix approach: Rename the table cases to full `Should...` subtest names and wrap the sparse queued task-number regression in a `Should...` subtest while preserving the same behavior assertions.
- Verification: Focused daemon snapshot tests passed, touched-package tests passed, and full `rtk make verify` passed after the production lint follow-up.
