---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/core/providerdefaults/defaults_test.go
line: 65
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22EB,comment:PRRC_kwDORy7nkc68_V6V
---

# Issue 006: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Use the required `t.Run("Should...")` subtest pattern in this new test.**

Line 13 introduces a standalone case; this repository requires subtest-based structure for test cases. Please wrap this scenario in `t.Run("Should ...")` (table-driven shape by default).

 

As per coding guidelines, `**/*_test.go`: “MUST use t.Run("Should...") pattern for ALL test cases” and “Use table-driven tests with subtests (t.Run) as the default pattern”.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/providerdefaults/defaults_test.go` around lines 13 - 65, Wrap
the body of
TestDefaultRegistryForWorkspaceRunsCodeRabbitCommandsFromWorkspaceRoot inside a
t.Run subtest (e.g., t.Run("Should fetch workspace-scoped CodeRabbit comments
from workspace root", func(t *testing.T) { ... })) so the test follows the
required t.Run("Should...") pattern; keep the existing setup (workspaceRoot,
fake gh script, env changes, registry := DefaultRegistryForWorkspace,
registry.Get, FetchReviews and assertions) but move them into the anonymous
subtest function and use that t parameter for assertions and t.Setenv/t.Chdir
calls.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:28cf3608-a263-4c0b-b9cd-af339b971c5f -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: The new workspace-root provider test is a standalone case instead of the required `t.Run("Should...")` structure.
- Fix plan: Wrap the body in a single `Should ...` subtest. This case will remain non-parallel because it uses `t.Setenv` and `t.Chdir`, which mutate process-wide state.
