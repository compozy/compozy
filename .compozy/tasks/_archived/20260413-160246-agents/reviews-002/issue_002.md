---
status: resolved
file: internal/core/agents/execution_test.go
line: 385
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56QRkz,comment:PRRC_kwDORy7nkc62z8SQ
---

# Issue 002: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# Check if the file exists and get its line count
wc -l internal/core/agents/execution_test.go

# Look at the structure of test functions
rg -n "^func Test" internal/core/agents/execution_test.go

# Check if any t.Run calls exist
rg -n "t.Run" internal/core/agents/execution_test.go
```

Repository: compozy/compozy

Length of output: 1881

---



</details>

**Refactor tests to use `t.Run("Should...")` subtests as the default pattern.**

All eight test functions in this file use a flat structure with direct assertions. Per the coding guidelines (`**/*_test.go`), Go tests must default to table-driven subtests with `t.Run("Should...")` for ALL test cases. This approach enables clearer failure messages, easier scenario grouping, and better alignment with the repository standard.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agents/execution_test.go` around lines 11 - 385, These tests
use flat top-level test functions instead of the repository standard
table-driven subtests; refactor each
TestResolveExecutionContextAppliesRuntimePrecedence,
TestResolveExecutionContextAppliesAgentModelWhenModelFlagIsUnset,
TestResolveExecutionContextPublicWrapperCapturesBaseRuntimeBeforeOverrides,
TestExecutionContextSystemPromptUsesCanonicalOrder,
TestExecutionContextSystemPromptKeepsDiscoveryCatalogCompact,
TestExecutionContextSystemPromptEscapesHostOwnedMetadataBlocks,
TestExecutionContextSystemPromptFallsBackToBasePromptWhenNoAgentSelected, and
TestResolveExecutionContextPreservesExistingRuntimeWhenAgentOmitsField to wrap
their assertions inside t.Run("Should <behaviour>", func(t *testing.T){ ... })
(or a table of named cases with t.Run for multiple scenarios), preserving
existing setup and assertions but moving them into appropriately named subtests
so failures report the descriptive subtest name and conform to the repository
testing pattern.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e61192d9-7c66-438c-8efd-0a27424736ab -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Notes:
  - This file contains single-scenario tests with clear top-level names; wrapping each one in a one-off `t.Run("Should ...")` block would be a mechanical style change, not a behavioral fix.
  - The repository guidance says subtests are the default pattern, but it does not require adding a redundant one-case subtest wrapper to every singleton test. The current structure already gives direct targeted test selection and clear failure locations.
  - No missing assertions, flaky behavior, or coverage gap was identified in `execution_test.go`, so I am not churning the file purely to satisfy a non-defect review preference.
  - Resolution: analysis complete; no code change required.
