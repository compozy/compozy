# Issue 16 - Review Thread Comment

**File:** `sdk/compozy/integration/hybrid_yaml_integration_test.go:61`
**Date:** 2025-11-01 01:57:03 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🛠️ Refactor suggestion_ | _🟠 Major_

**Conform to required t.Run usage.**

This test should also wrap its assertions inside `t.Run("Should …", func(t *testing.T) { ... })` to meet the testing convention set in the guidelines. Please update the structure accordingly. As per coding guidelines

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/integration/hybrid_yaml_integration_test.go around lines 20-61,
wrap the existing test body inside t.Run so assertions run in a named subtest;
replace the current top-level statements with t.Run("Should validate hybrid YAML
integration", func(t *testing.T) { /* existing body here */ }) and ensure all
test helpers (lifecycleTestContext, require.*, assert.*) use the inner t; keep
setup, file creation, engine.LoadWorkflowsFromDir, ValidateReferences and
assertions unchanged but moved into the inner closure.
```

</details>

<!-- fingerprinting:phantom:medusa:sabertoothed -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa2p`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa2p
```

---
*Generated from PR review - CodeRabbit AI*
