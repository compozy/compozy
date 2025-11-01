# Issue 19 - Review Thread Comment

**File:** `sdk/compozy/integration/standalone_integration_test.go:37`
**Date:** 2025-10-31 14:57:19 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Wrap the integration test in a `t.Run("Should …")` subtest.**

Project guidelines require every Go test to organize its cases with `t.Run("Should describe behavior", …)`. This function runs assertions directly and skips the mandated naming pattern, making the suite fail the convention check. Please wrap the body in a subtest (and consider `t.Parallel()` inside if appropriate) so it follows the required structure.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/integration/standalone_integration_test.go around lines 16 to 37,
the test function TestStandaloneIntegrationLifecycle contains assertions run
directly and must be wrapped in a t.Run subtest to meet project conventions;
modify the function so its existing body is executed inside t.Run("Should
<describe behavior>", func(t *testing.T) { ... }), optionally add t.Parallel()
at the top of the subtest if parallel execution is appropriate, and keep the
outer TestStandaloneIntegrationLifecycle signature unchanged while moving
cleanup and assertions into the subtest closure.
```

</details>

<!-- fingerprinting:phantom:medusa:sabertoothed -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gJFFJ`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gJFFJ
```

---
*Generated from PR review - CodeRabbit AI*
