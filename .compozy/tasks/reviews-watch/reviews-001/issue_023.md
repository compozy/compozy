---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/store/globaldb/archive_test.go
line: 196
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22Eq,comment:PRRC_kwDORy7nkc68_V7K
---

# Issue 023: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Convert the two new tests to the required `t.Run("Should...")` subtest format.**

These scenarios are valuable, but the newly added tests should follow the enforced table-driven/subtest convention.

 

As per coding guidelines, `**/*_test.go`: “MUST use t.Run("Should...") pattern for ALL test cases” and “Use table-driven tests with subtests (t.Run) as the default pattern”.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/globaldb/archive_test.go` around lines 100 - 196, The two new
tests violate the project's subtest/table-driven convention—convert
TestGetWorkflowArchiveEligibilityAllowsResolvedReviewOnlyWorkflow and
TestGetWorkflowArchiveEligibilityBlocksUnresolvedReviewOnlyWorkflow into
subtests under a single TestGetWorkflowArchiveEligibility function that uses
t.Run("Should ...") for each case; preserve t.Parallel(), drive cases via a
table/slice (each case including name, workflowID, review rounds, and expected
counts/archivable/skipReason), and inside each subtest call
mustReconcileArchiveWorkflow, db.GetWorkflowArchiveEligibility, and assert
TaskTotal/ReviewIssueTotal/UnresolvedReviewIssues/Archivable()/SkipReason()
accordingly so the unique helpers (mustReconcileArchiveWorkflow, mustWorkspace,
openTestGlobalDB, GetWorkflowArchiveEligibility) remain referenced and tests
follow the required pattern.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:28cf3608-a263-4c0b-b9cd-af339b971c5f -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: The two new archive-eligibility cases are real coverage, but they do not follow the repository’s required subtest naming and table-driven structure. I will consolidate them under a `t.Run("Should ...")` table without weakening the assertions.
- Resolution: Consolidated the review-only archive-eligibility coverage into the required table-driven `Should ...` subtest form and reverified it through `make verify`.
