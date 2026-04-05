# Issue 8 - Review Thread Comment

**File:** `internal/core/run/exec_flow_integration_test.go:333`
**Date:** 2026-04-05 19:22:18 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** The table-driven test cases already use `t.Run(tc.name, ...)`, and the current labels do not follow the repo’s enforced `Should...` naming convention. Renaming the cases is mechanical and correct.

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Rename these subtests to the `Should...` convention.**

The cases are already table-driven and parallelized; only the `t.Run` labels are out of line with the repo’s enforced test pattern.

As per coding guidelines, `MUST use t.Run("Should...") pattern for ALL test cases`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/exec_flow_integration_test.go` around lines 298 - 333,
Rename the subtest names in the table-driven test to follow the "Should..."
convention: update the t.Run labels for each case (currently values like "keeps
agent message chunks", "drops plan updates", etc.) to "Should keep agent message
chunks", "Should drop plan updates", "Should keep unknown completed updates",
and "Should drop unknown running updates" so they match the repo's enforced
t.Run("Should...") pattern; ensure these updated labels are used in the loop
that calls t.Run for the tests exercising shouldEmitLeanSessionUpdate.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5606214d-8719-43d5-98a2-064a6cd78ca1 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc547tHm`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc547tHm
```

---
*Generated from PR review - CodeRabbit AI*
