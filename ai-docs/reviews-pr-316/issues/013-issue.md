# Issue 13 - Review Thread Comment

**File:** `sdk/compozy/cleanup_test.go:48`
**Date:** 2025-10-31 14:57:18 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Restructure this test into `t.Run("Should …")` subtests.**

Per the testing guidelines, every Go test must express behaviors via `t.Run("Should …", …)` subtests. This function runs multiple scenarios in a single block with no subtests, so it doesn’t meet the requirement. Break the success and failure paths into clearly named `t.Run("Should …")` blocks (and use testify assertions if they aren’t already in place) to align with the mandated pattern.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/cleanup_test.go around lines 24 to 48, the test currently
exercises multiple scenarios in one function; split it into t.Run("Should ...")
subtests for each behavior: one subtest that verifies cleanupModeResources
returns an error and increments called twice, one subtest that exercises
cleanupStore (success path), and one subtest that sets engine.project and a
failingStore then asserts RegisterTool returns an error and that engine.tools is
empty; move the relevant setup and assertions into clearly named t.Run blocks
(using the existing require/assert calls) so each behavior is isolated and
self-describing.
```

</details>

<!-- fingerprinting:phantom:medusa:sabertoothed -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gJFEh`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gJFEh
```

---
*Generated from PR review - CodeRabbit AI*
