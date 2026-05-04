---
status: resolved
file: internal/core/run/ui/update_test.go
line: 249
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4135484321,nitpick_hash:599a962a4df2
review_hash: 599a962a4df2
source_review_id: "4135484321"
source_review_submitted_at: "2026-04-19T04:55:43Z"
---

# Issue 004: Wrap the new scenarios in t.Run("Should...") subtests.
## Review Comment

These added cases are still written as standalone top-level tests, which doesn't match the repository’s required test shape for new scenarios. As per coding guidelines, "MUST use t.Run("Should...") pattern for ALL test cases".

Also applies to: 353-385, 579-620

## Triage

- Decision: `invalid`
- Root cause: the review comment assumes a repository-wide requirement that every new scenario be wrapped as `t.Run("Should...")`, but that rule does not exist in the scoped workspace guidance (`AGENTS.md` / `CLAUDE.md`) and it does not match the surrounding package conventions. `internal/core/run/ui/update_test.go` and sibling UI test files predominantly use standalone top-level `Test...` functions for single-scenario coverage, including the cases around the flagged lines.
- Resolution: no code change was made. Rewriting only these three standalone tests into nested `Should...` subtests would be style churn without improving behavior, coverage, or consistency inside `internal/core/run/ui`.
- Verification: `make verify` (`0` lint issues, `DONE 2416 tests, 1 skipped`, build succeeded).
