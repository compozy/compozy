---
status: resolved
file: internal/core/extension/discovery_test.go
line: 13
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093239578,nitpick_hash:54fec9d640b7
review_hash: 54fec9d640b7
source_review_id: "4093239578"
source_review_submitted_at: "2026-04-11T02:29:46Z"
---

# Issue 020: Consider adding t.Parallel() to independent tests.
## Review Comment

These discovery tests appear to be independent since they each create their own temporary directories. Adding `t.Parallel()` would improve test execution time.

## Triage

- Decision: `invalid`
- Notes: These discovery tests all call `withVersion`, which mutates the package-global `version.Version` during each test. Adding blanket top-level `t.Parallel()` here would create data-race and cleanup-order hazards unless the version override mechanism is redesigned first. The suggestion is therefore not safe as written.
