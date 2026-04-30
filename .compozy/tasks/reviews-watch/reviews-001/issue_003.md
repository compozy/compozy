---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/core/archive_test.go
line: 264
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22Dx,comment:PRRC_kwDORy7nkc68_V6C
---

# Issue 003: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Adopt table-driven `t.Run("Should ...")` subtests for the new review-only archive cases.**

These new tests are valid functionally, but they should follow the project’s required subtest pattern (and use `t.Parallel()` where independence is preserved).

 

As per coding guidelines, "Use table-driven tests with subtests (`t.Run`) as the default pattern" and "MUST use t.Run("Should...") pattern for ALL test cases".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/archive_test.go` around lines 228 - 264, Refactor the two tests
TestArchiveTaskWorkflowAllowsResolvedReviewOnlyWorkflow and
TestArchiveTaskWorkflowRejectsUnresolvedReviewOnlyWorkflow into a single
table-driven test using t.Run("Should ...") subtests: create a slice of cases
describing name ("Should archive resolved review-only", "Should reject
unresolved review-only"), inputs (use writeArchiveReviewRound to write rounds
and mustSyncArchiveWorkflow), expected error (nil vs
globaldb.ErrWorkflowNotArchivable) and expected result assertions
(WorkflowsScanned/Archived/Skipped and os.Stat checks), then iterate cases and
call t.Run(case.name, func(t *testing.T) { t.Parallel(); /* call
Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir}) and assert
*/ }); ensure each subtest uses t.Parallel() where safe and reference Archive,
ArchiveConfig, writeArchiveReviewRound, and mustSyncArchiveWorkflow to locate
logic.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:9d503b4c-1a51-4ef5-a14d-2e16d6ffd95a -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: The two new review-only archive scenarios were added as standalone tests, but project guidance requires table-driven subtests with `t.Run("Should...")` for test cases.
- Fix plan: Consolidate the resolved/unresolved review-only archive coverage into one table-driven test with `Should ...` subtests and preserve the existing assertions.
