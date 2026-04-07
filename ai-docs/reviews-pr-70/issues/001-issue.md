# Issue 1 - Review Thread Comment

**File:** `internal/core/agent/registry_test.go:695`
**Date:** 2026-04-07 15:21:57 UTC
**Status:** - [x] RESOLVED

## Technical Disposition

`VALID` - the review-targeted registry tests were still standalone functions. They now wrap their assertions in `t.Run("Should ...")` subtests in `internal/core/agent/registry_test.go`, preserving the existing coverage while matching the repository test pattern.

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Adopt the required `t.Run("Should...")` subtest pattern for the new tests.**

The newly added test cases are standalone functions; project policy requires test cases to be organized with `t.Run("Should...")` (table-driven by default).

As per coding guidelines, "MUST use t.Run("Should...") pattern for ALL test cases" and "Use table-driven tests with subtests (`t.Run`) as the default pattern for Go tests."

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/registry_test.go` around lines 587 - 695, These tests
violate the project's t.Run("Should...") subtest pattern—refactor each
standalone test (TestValidateRuntimeConfigRejectsNilConfig,
TestEnsureAvailableRejectsNilConfig,
TestValidateRuntimeConfigAcceptsResolvedPromptTextAsExecPromptSource,
TestBuildShellCommandStringShellEscapesInterpolatedArguments,
TestEnsureAvailableHonorsCallerContext,
TestJoinAvailabilityErrorsPreservesAvailabilityErrorTypes) to use the required
subtest pattern: either convert them into a single table-driven test with
multiple t.Run("Should ...", func(t *testing.T){ ... }) entries or wrap the
existing body inside a t.Run("Should <behavior>", func(t *testing.T){ ... })
call; keep the existing assertions and setup (e.g., calls to
ValidateRuntimeConfig, EnsureAvailable, BuildShellCommandString,
joinAvailabilityErrors, registerTestSpec) unchanged but move them into the
corresponding subtest functions and preserve t.Parallel where appropriate inside
each subtest.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:1c95559b-1220-4d4f-8066-bd9bc9b6b6b3 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55UjZG`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55UjZG
```

---
*Generated from PR review - CodeRabbit AI*
