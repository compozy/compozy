# Issue 3 - Review Thread Comment

**File:** `internal/setup/reusable_agent_sources_test.go:26`
**Date:** 2026-04-13 19:08:30 UTC
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Assert the failure reason, not just `err != nil`.**

These new negative-path tests pass on any error, so a regression in the actual validation path could still look green. Please check the expected message/content for each case so the tests prove the intended guardrail fired.

As per coding guidelines `**/*_test.go`: `MUST have specific error assertions (ErrorContains, ErrorAs)`.

Also applies to: 28-55, 57-72, 114-129

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/setup/reusable_agent_sources_test.go` around lines 12 - 26, The
tests only assert err != nil which is too weak; update
TestResolveDeclaredSkillPackSource and TestResolveExtensionReusableAgentSource
to assert the specific failure message/content instead (e.g., use
require.ErrorContains/require.ErrorAs or assert.ErrorContains against the
expected validation text) so the checks verify the intended guardrail from
resolveDeclaredSkillPackSource and resolveExtensionReusableAgentSource; apply
the same change to the other negative-path tests in this file noted in the
comment.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:f0410b06-af06-4312-8bf8-f831ab4cc296 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: VALID
- Resolution: strengthened the negative-path reusable-agent source tests to assert the actual validation message, and added a traversal case that proves `parseReusableAgent` rejects invalid names before attempting to read the bundle.

## Resolve

Thread ID: `PRRT_kwDORy7nkc56n9hT`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc56n9hT
```

---

_Generated from PR review - CodeRabbit AI_
