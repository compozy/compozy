---
status: resolved
file: internal/core/extension/review_provider_bridge_integration_test.go
line: 26
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093952670,nitpick_hash:d2b43282960a
review_hash: d2b43282960a
source_review_id: "4093952670"
source_review_submitted_at: "2026-04-11T16:18:34Z"
---

# Issue 013: Collapse the Go/TS permutations into table-driven subtests.
## Review Comment

These six scenarios are the same harness with different runtime/expected-error permutations. Folding them into `t.Run("Should ...")` cases would cut duplication and align the file with the repo’s Go test conventions.

As per coding guidelines, "Use table-driven tests with subtests (`t.Run`) as the default pattern" and "MUST use t.Run("Should...") pattern for ALL test cases".

## Triage

- Decision: `valid`
- Notes:
  - Root cause: the Go and TypeScript bridge tests repeat the same fetch/resolve harness across six permutations, which obscures the actual scenario differences and misses the repo’s `Should...` table-driven test convention.
  - Fix plan: consolidate the permutations into table-driven `Should...` subtests while keeping the same real-stdio assertions for success and contract failures.
  - Resolved: consolidated the bridge permutations into table-driven `Should...` subtests in `internal/core/extension/review_provider_bridge_integration_test.go`; verified with `make verify`.
