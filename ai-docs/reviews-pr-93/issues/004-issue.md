# Issue 4 - Review Thread Comment

**File:** `internal/setup/bundle_test.go:71`
**Date:** 2026-04-13 18:25:17 UTC
**Status:** - [ ] UNRESOLVED

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Please align new test cases with the required `t.Run("Should...")` style.**

These newly added cases should follow the mandated `Should...` subtest convention for consistency with the repository test policy.

As per coding guidelines `**/*_test.go`: `MUST use t.Run("Should...") pattern for ALL test cases`.

Also applies to: 73-83

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/setup/bundle_test.go` around lines 61 - 71, The test
TestListBundledReusableAgentsAllowsEmptyRoster should be converted to use a
t.Run subtest with the mandated "Should..." description; wrap the existing
assertions inside t.Run("Should return empty bundled reusable-agent roster when
none exist", func(t *testing.T) { ... }) and keep the body calling
ListBundledReusableAgents and the same checks, and apply the same change to the
other test that follows (the test covering the non-empty roster), ensuring both
tests use t.Run("Should...") names while preserving calls to
ListBundledReusableAgents and the same error/assert logic.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:36f465d2-9fc4-45e9-b8b2-cc8607599611 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: VALID
- Resolution: converted the bundled reusable-agent empty-roster tests to `t.Run("Should ...")` subtests without changing their assertions.

## Resolve

Thread ID: `PRRT_kwDORy7nkc56nVL6`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc56nVL6
```

---

_Generated from PR review - CodeRabbit AI_
