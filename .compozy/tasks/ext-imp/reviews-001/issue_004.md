---
status: resolved
file: internal/cli/root_command_execution_test.go
line: 822
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093952670,nitpick_hash:9c5416859715
review_hash: 9c5416859715
source_review_id: "4093952670"
source_review_submitted_at: "2026-04-11T16:18:34Z"
---

# Issue 004: Consider table-driving these workflow-output cases.
## Review Comment

These three tests use the same harness and mostly vary by args plus expected stream shape. Folding them into a `t.Run(...)` table would remove a lot of duplication, and the subtests can call `t.Parallel()` because the helper mutexes already serialize the shared cwd/stdout mutations.

As per coding guidelines: "Use table-driven tests with subtests (`t.Run`) as the default pattern" and "Use `t.Parallel()` for independent subtests."

Also applies to: 935-970, 1045-1108

## Triage

- Decision: `invalid`
- Notes:
  - The referenced tests share some harness helpers, but they validate materially different command flows, persisted artifacts, and stream envelopes. Folding them into one table would trade away readability without improving behavioral coverage.
  - `withWorkingDir` intentionally holds a process-wide mutex until cleanup, so `t.Parallel()` would not yield meaningful concurrency here; it would only complicate failure output for little value.
