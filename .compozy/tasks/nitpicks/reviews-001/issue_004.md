---
status: resolved
file: internal/core/fetch_test.go
line: 304
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56KkL-,comment:PRRC_kwDORy7nkc62sRjP
---

# Issue 004: _🛠️ Refactor suggestion_ | _🟠 Major_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_

**Collapse these nitpick-history cases into table-driven `t.Run("Should ...")` subtests.**

The three scenarios are the same behavior matrix with different fixtures, so a table-driven test will keep this easier to extend and bring it back in line with the repo’s required Go test pattern.

As per coding guidelines, "`**/*_test.go`: Use table-driven tests with subtests (`t.Run`) as the default pattern for Go tests", "Use `t.Parallel()` for independent Go test subtests", and "MUST use `t.Run(\"Should...\")` pattern for ALL test cases".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/fetch_test.go` around lines 112 - 304, Collapse the three tests
(TestFetchReviewsSkipsResolvedStaleNitpickHashes,
TestFetchReviewsReimportsUnresolvedNitpickHashes,
TestFetchReviewsReimportsResolvedNitpickWhenProviderReviewIsNewer) into a single
table-driven test that iterates cases and uses t.Run("Should ...") subtests; for
each case call t.Parallel() and perform the per-case setup (tmpDir, prdDir,
writeHistoricalNitpickRound, overriding defaultProviderRegistry, and t.Cleanup)
inside the subtest so fixtures are isolated, then call fetchReviews and assert
result.Total per case. Keep the same unique symbols (fetchReviews,
writeHistoricalNitpickRound, defaultProviderRegistry, stubReviewProvider) to
locate and reuse existing setup/fixtures when building the table entries.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:3c6a12e0-accd-4d1e-a3db-5a1d78606f67 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: The three nitpick-history tests repeat the same provider/round fixture setup and assert a small behavior matrix in separate top-level tests instead of expressing the scenarios as isolated subtests.
- Impact: The duplication makes the history behavior harder to extend, and it is a poor fit for the repository's default table-driven `t.Run("Should ...")` pattern for Go tests.
- Fix approach: Replace the duplicated top-level tests with one table-driven test that builds fixtures per case inside `t.Run("Should ...")` subtests, while preserving isolation and covering the same regression matrix.
- Resolution: The nitpick-history regressions now run through one table-driven `t.Run("Should ...")` matrix that covers stale resolved nitpicks, unresolved nitpicks, newer timestamps, and same-second newer review IDs.
- Verification: `go test ./internal/cli ./internal/core ./internal/core/provider/coderabbit` passed, and `env -u COMPOZY_NO_UPDATE_NOTIFIER make verify` passed cleanly.
