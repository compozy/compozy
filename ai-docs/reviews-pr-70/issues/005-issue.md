# Issue 5 - Review Thread Comment

**File:** `internal/core/model/preparation_test.go:62`
**Date:** 2026-04-07 11:44:37 America/Sao_Paulo
**Status:** - [x] ADDRESSED

## Technical Disposition

`VALID` - the preparation tests were still top-level only. They now run through explicit `t.Run("Should ...")` subtests while preserving the original assertions and behavior.

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Wrap test cases in `t.Run("Should...")` subtests (table-driven default).**

These cases are currently top-level only. Please move them into `t.Run("Should...")` subtests (preferably table-driven) to align with the repo’s required Go test pattern.


As per coding guidelines, "`**/*_test.go`: Use table-driven tests with subtests (`t.Run`) as the default pattern for Go tests" and "MUST use `t.Run(\"Should...\")` pattern for ALL test cases".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/model/preparation_test.go` around lines 13 - 62, Split the two
top-level tests TestSolvePreparationSetJournalPreservesExistingOwnership and
TestSolvePreparationCloseJournalPreservesHandleOnFailure into t.Run subtests
(preferably table-driven): create a table of cases describing the scenario and
expected outcome and for each case call t.Run("Should <description>", func(t
*testing.T){ ... }); move the existing setup/assert logic into the subtest body
and preserve use of SolvePreparation.SetJournal, SolvePreparation.Journal(),
CloseJournal, and the stubJournalHandle (err) to drive the failure case; ensure
each subtest calls t.Parallel() where appropriate and that the original
assertions remain unchanged.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:5863e1b4-cf18-4de0-81c6-bd40921cf292 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55T4L1`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55T4L1
```

---
*Generated from PR review - CodeRabbit AI*
