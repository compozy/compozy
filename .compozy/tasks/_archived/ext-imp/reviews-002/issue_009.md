---
status: resolved
file: sdk/create-extension/test/create-extension.test.ts
line: 27
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4094514795,nitpick_hash:ef464c25a408
review_hash: ef464c25a408
source_review_id: "4094514795"
source_review_submitted_at: "2026-04-12T04:17:10Z"
---

# Issue 009: Build the local workspaces once per file.
## Review Comment

These tests rebuild the same two packages four times. Caching that promise once would cut CI time and reduce churn around shared `dist` outputs without changing the assertions.

Also applies to: 46-46, 68-68, 91-91

## Triage

- Decision: `valid`
- Notes:
  - Root cause: every integration-style test in this file calls `buildLocalPackages()`, which rebuilds the same two workspaces repeatedly.
  - Impact: redundant package builds add unnecessary CI time and extra churn around shared `dist` outputs without increasing coverage.
  - Fix approach: memoize the package-build promise once per test file and reuse it across the tests.
  - Implemented: memoized the local workspace build as a shared promise so the file builds the two packages once and reuses the result across all tests.
  - Verification: targeted Vitest coverage passed and the final `make verify` run passed cleanly.
