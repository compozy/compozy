---
status: resolved
file: internal/core/reviews/parser_test.go
line: 96
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZWr,comment:PRRC_kwDORy7nkc61XmRS
---

# Issue 019: _🛠️ Refactor suggestion_ | _🟠 Major_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_

**Use the required `t.Run("Should...")` subtest shape here.**

These cases are currently packed into broad top-level tests instead of the repo’s default table-driven subtest pattern, so failures are harder to localize and the new suite does not follow the enforced test structure. As per coding guidelines, Use table-driven tests with subtests (`t.Run`) as the default pattern for Go tests; MUST use `t.Run("Should...")` pattern for ALL test cases.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/reviews/parser_test.go` around lines 9 - 96, The tests in
TestReviewParsingHelpers, TestExtractIssueNumber, and
TestWrapParseErrorProvidesMigrationGuidance must be refactored into table-driven
subtests using t.Run("Should ...") so each scenario is isolated; create a slice
of test cases (with name, input, expected) for the
ParseReviewContext/IsReviewResolved/legacy parse/extract flows and for
ExtractIssueNumber and WrapParseError, iterate over cases and call t.Run("Should
<behavior>", func(t *testing.T){ t.Parallel(); ... assertions ... }), moving
each assertion into its matching subtest and invoking the existing helper
functions (ParseReviewContext, IsReviewResolved, ParseLegacyReviewContext,
ExtractLegacyReviewBody, LooksLikeLegacyReviewFile, ExtractIssueNumber,
WrapParseError) inside the subtests; ensure error expectations use errors.Is
where appropriate and keep test behaviour identical while adopting the required
"Should..." subtest naming and table-driven pattern.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:a60f1eb0-d795-4bb2-8b4d-2afc11c2fe85 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  The current parser tests are broad top-level cases, which makes failures less localized and does not follow the repository’s required table-driven subtest style. This is a valid test-structure issue in the scoped test file. I will refactor the existing scenarios into `t.Run("Should ...")` cases while preserving their coverage and adding a regression case for scoped legacy XML parsing.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.
