---
status: resolved
file: internal/core/agents/agents_test.go
line: 506
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5tq,comment:PRRC_kwDORy7nkc62zc8W
---

# Issue 008: _🛠️ Refactor suggestion_ | _🟠 Major_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_

**Refactor this suite to the repo’s table-driven `t.Run("Should...")` pattern.**

The scenarios are solid, but this file adds a large batch of standalone tests instead of the required subtest structure. Converting them to table-driven cases would also remove a lot of repeated fixture/setup code here.


As per coding guidelines "Use table-driven tests with subtests (`t.Run`) as the default test pattern" and "MUST use t.Run(\"Should...\") pattern for ALL test cases".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agents/agents_test.go` around lines 14 - 506, Multiple
standalone tests (e.g., TestDiscoverParsesValidAgentDefinition,
TestDiscoverRejectsReservedAndInvalidAgentNames,
TestDiscoverRejectsUnsupportedDeferredMetadataFields, etc.) must be converted to
a single table-driven test using t.Run("Should ...") subtests; refactor by
creating a TestDiscover (or consolidate within an existing TestDiscover*) that
defines a slice of cases (name, setup parameters like homeDir/workspaceRoot,
agent files via writeWorkspaceAgent/writeGlobalAgent, expected
catalog/Problems/assertions) and loops over them calling t.Run("Should
<behavior>", func(t *testing.T){ /* run per-case setup using newTestRegistry,
call registry.Discover, and perform the existing assertions */ }); reuse helpers
newTestRegistry, writeWorkspaceAgent, writeGlobalAgent, validAgentMarkdown, and
problemsByName to remove duplicated setup and preserve each original case's
assertions inside the corresponding subtest.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d740b4bc-0bac-4faf-9dba-d2618b9a24f6 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Analysis: This is a refactor/style suggestion rather than a correctness defect. The existing test cases already isolate distinct discovery behaviors and remained easy to diagnose during remediation.
- Why no change: Rewriting the suite into one large table-driven test would create unrelated churn without addressing a real bug or regression risk.
- Evidence: inspected the current `internal/core/agents/agents_test.go` coverage and reran `go test ./internal/core/agents/...`
