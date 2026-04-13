# Issue 3 - Review Thread Comment

**File:** `internal/core/extension/discovery_test.go:344`
**Date:** 2026-04-13 18:25:17 UTC
**Status:** - [ ] UNRESOLVED

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Adopt the required `t.Run("Should...")` pattern for this new test case.**

Wrap the test body in a `t.Run("Should ...")` subtest (and keep it parallel where appropriate) to align with the repository’s enforced test structure.

As per coding guidelines `**/*_test.go`: `MUST use t.Run("Should...") pattern for ALL test cases`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/discovery_test.go` around lines 311 - 344, Wrap the
existing TestDiscoveryExtractsReusableAgentsFromEnabledExtensions body in a
t.Run subtest using the "Should ..." naming convention (e.g., t.Run("Should
extract reusable agents from enabled extensions", func(t *testing.T) { ... })),
move the test logic into that function, and call t.Parallel() inside the subtest
if other tests are parallelized; ensure the outer
TestDiscoveryExtractsReusableAgentsFromEnabledExtensions remains as the test
function and only contains the t.Run invocation so it conforms to the
repository's t.Run("Should...") pattern.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:36f465d2-9fc4-45e9-b8b2-cc8607599611 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: VALID
- Resolution: wrapped the reusable-agent discovery test in `t.Run("Should extract reusable agents from enabled extensions", ...)`.

## Resolve

Thread ID: `PRRT_kwDORy7nkc56nVL0`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc56nVL0
```

---

_Generated from PR review - CodeRabbit AI_
