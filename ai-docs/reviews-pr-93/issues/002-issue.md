# Issue 2 - Review Thread Comment

**File:** `internal/core/extension/assets_test.go:135`
**Date:** 2026-04-13 18:25:17 UTC
**Status:** - [ ] UNRESOLVED

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Please wrap this new test in the required `t.Run("Should...")` pattern.**

The test logic is good; only the enforced test-case structure is missing.

As per coding guidelines `**/*_test.go`: `MUST use t.Run("Should...") pattern for ALL test cases`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/assets_test.go` around lines 91 - 135, The test
function TestExtractDeclaredReusableAgentsResolvesAbsolutePaths must be wrapped
in a t.Run subtest using the "Should..." pattern; update the test so its body is
inside t.Run("Should resolve declared reusable agents to absolute paths", func(t
*testing.T) { t.Parallel(); ... }) keeping the existing assertions and calls to
ExtractDeclaredReusableAgents and writeTestFile intact so the test logic is
unchanged but conforms to the required t.Run("Should...") structure.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:36f465d2-9fc4-45e9-b8b2-cc8607599611 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: VALID
- Resolution: wrapped the test body in `t.Run("Should resolve declared reusable agents to absolute paths", ...)` and kept the existing assertions intact.

## Resolve

Thread ID: `PRRT_kwDORy7nkc56nVLx`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc56nVLx
```

---

_Generated from PR review - CodeRabbit AI_
