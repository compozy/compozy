# Issue 12 - Review Thread Comment

**File:** `sdk/compozy/app_test.go:50`
**Date:** 2025-10-31 14:57:18 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**LGTM! Well-structured constructor tests.**

The tests properly validate the `New` constructor with:
- Correct use of `t.Context()` (not `context.Background()`)
- Proper use of `t.Parallel()` for parallel execution
- Good coverage of success and error cases
- Appropriate use of `require.NoError` and `assert` for validations

All functions are within the 50-line limit and follow good testing practices.



**Minor suggestion:** Consider using nested `t.Run("Should...")` subtests for consistency with the coding guidelines' preferred test pattern, though the current structure is also acceptable.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/app_test.go around lines 14 to 50, the reviewer suggests
switching these top-level tests to nested t.Run subtests for consistency with
project guidelines; update each test to be a t.Run("Should <description>",
func(t *testing.T) { t.Parallel(); ... }) subtest (preserve the existing setup,
assertions, use of t.Context(), require/assert calls, and t.Parallel() inside
each subtest) so they run as named subtests while keeping behavior identical.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gJFEc`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gJFEc
```

---
*Generated from PR review - CodeRabbit AI*
