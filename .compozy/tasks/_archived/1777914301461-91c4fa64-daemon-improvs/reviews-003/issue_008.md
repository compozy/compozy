---
status: resolved
file: internal/store/rundb/run_db_test.go
line: 447
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58jijg,comment:PRRC_kwDORy7nkc655NTG
---

# Issue 008: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**New tests should use subtests (`t.Run("Should...")`) per test policy.**

Both new test functions are single-flow top-level tests without `t.Run("Should...")` subtests/table-driven structure required by the repository test rules.



As per coding guidelines, "Use table-driven tests with subtests (`t.Run`) as the default pattern" and "MUST use t.Run(\"Should...\") pattern for ALL test cases".


Also applies to: 449-508

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/rundb/run_db_test.go` around lines 365 - 447, The test
TestRunDBUpsertIntegrityIsStickyAndSurvivesReopen violates the repo policy
requiring subtests; wrap the test body in a t.Run("Should ...") subtest (e.g.
t.Run("Should persist integrity and sticky counts across reopen", func(t
*testing.T) { ... })) and similarly update the other new test referenced (lines
449-508) to use t.Run subtests (or table-driven t.Run cases if there are
multiple scenarios); keep the existing assertions and helper calls
(openTestRunDB, UpsertIntegrity, GetIntegrity, Open, Close) unchanged but move
them inside the t.Run closures and call t.Parallel() inside the subtest where
appropriate.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:4bbba011-c695-41c9-94a6-2c134f46c6e5 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `INVALID`
- Reasoning: `run_db_test.go` predominantly uses top-level scenario tests, and the two flagged tests each validate a single behavioral contract with no repeated case matrix that would benefit from an extra `t.Run` wrapper.
- Root cause: The suggestion asks for ceremonial nesting rather than fixing a correctness, isolation, or maintenance problem in the current tests.
- Resolution plan: No code change. Keep the tests as focused top-level scenarios.

## Resolution

- Closed as `invalid`. The existing top-level scenario tests remain in place because the requested one-case `t.Run` wrappers would be ceremonial only.

## Verification

- Confirmed against the current file state and completed a fresh `make verify` pass after the in-scope fixes for the valid issues.
