---
status: resolved
file: internal/core/agents/mcpserver/server_test.go
line: 187
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56QRk2,comment:PRRC_kwDORy7nkc62z8ST
---

# Issue 004: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# Check if the file exists at the specified path
find . -path "*internal/core/agents/mcpserver/server_test.go" -type f
```

Repository: compozy/compozy

Length of output: 1881

---

🏁 Script executed:

```shell
# Count the lines in the file to verify it matches the line numbers
if [ -f "internal/core/agents/mcpserver/server_test.go" ]; then
  wc -l "internal/core/agents/mcpserver/server_test.go"
fi
```

Repository: compozy/compozy

Length of output: 1881

---

🏁 Script executed:

```shell
# Check the actual content around lines 21-187
if [ -f "internal/core/agents/mcpserver/server_test.go" ]; then
  sed -n '21,187p' "internal/core/agents/mcpserver/server_test.go" | head -50
fi
```

Repository: compozy/compozy

Length of output: 1881

---



</details>

**Reshape this test file into `t.Run("Should...")` subtests.**

All test functions must follow the required t.Run pattern with "Should..." naming convention. Currently, all tests use top-level `func TestXxx(t *testing.T)` pattern without subtests, which does not comply with the repository's Go test guidelines.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agents/mcpserver/server_test.go` around lines 21 - 187, Convert
the top-level tests into t.Run subtests with "Should..." names: create a single
Test... wrapper (e.g., TestServerBehaviors) and move the bodies of
TestLoadHostContextFromEnvParsesReservedPayload,
TestLoadHostContextFromEnvRejectsMissingPayload,
TestRunAgentToolMarksStructuredFailuresAsToolErrors,
TestNewServerAppliesOptionsAndDefaults,
TestLoadHostContextFromEnvUsesProcessEnvironment, and
TestServeStdioReturnsWhenContextIsCanceled into individual t.Run("Should ...",
func(t *testing.T){ ... }) blocks (keep t.Parallel() where used), rename each
subtest description to start with "Should" (e.g., "Should parse reserved payload
from env"), and remove the original top-level TestXxx functions so the file uses
the required subtest pattern while preserving all assertions and helper setup
(e.g., stdioSwapMu, ServeStdio, LoadHostContextFromEnv, NewServer,
runAgentTool).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e61192d9-7c66-438c-8efd-0a27424736ab -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Notes:
  - This comment is another request for a mechanical `t.Run("Should ...")` rewrite, not a defect report. The current file already isolates behaviors with descriptive top-level tests, and targeted execution remains straightforward.
  - Forcing all of these into a single umbrella test would add indirection and complicate the stdio-swapping test in this file without improving hermeticity or coverage.
  - I found one real behavior issue in this file (`issue_003`), but the existing test structure itself is not broken.
  - Resolution: analysis complete; no code change required.
