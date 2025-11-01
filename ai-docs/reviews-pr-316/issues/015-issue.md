# Issue 15 - Review Thread Comment

**File:** `sdk/compozy/integration/distributed_integration_test.go:89`
**Date:** 2025-11-01 01:57:02 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🛠️ Refactor suggestion_ | _🟠 Major_

**Use mandated subtest structure.**

Guidelines require wrapping integration test logic in a `t.Run("Should …", func(t *testing.T) { ... })` subtest. Please restructure this test accordingly so it follows the prescribed pattern. As per coding guidelines

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/integration/distributed_integration_test.go around lines 24-89,
the test function body must be wrapped in a mandated subtest; refactor
TestDistributedIntegrationLifecycle to call t.Run with a descriptive name (e.g.,
"Should run distributed integration lifecycle") and move all existing test logic
into the subtest anonymous func(t *testing.T) so the setup, assertions, cleanup,
and server checks execute inside t.Run while the outer
TestDistributedIntegrationLifecycle only invokes that subtest.
```

</details>

<!-- fingerprinting:phantom:medusa:sabertoothed -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa2n`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa2n
```

---
*Generated from PR review - CodeRabbit AI*
