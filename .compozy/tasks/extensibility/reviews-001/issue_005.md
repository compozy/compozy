---
status: resolved
file: internal/cli/extension/enablement.go
line: 63
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVOu,comment:PRRC_kwDORy7nkc621Vae
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Enable/disable is resolving the winner, not the toggle target.**

With duplicate names across scopes, `findEffectiveExtension(result, name)` selects the currently effective entry. That means `ext enable <name>` cannot turn on a disabled workspace/user override if a lower-precedence bundled entry is currently winning; the command will target the bundled entry instead. Resolve toggle targets from the discovered matches, not just the effective set.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/extension/enablement.go` around lines 57 - 63, The current code
uses findEffectiveExtension(result, name) which returns the winning entry,
causing ext enable/disable to target the effective winner instead of the
specific match the user intended; change the logic to search the discovered
matches in result for the highest-precedence entry whose Enabled state differs
from the requested enable value and pass that entry to toggleEntry(ctx,
env.store, entry, enable). Concretely, replace the single findEffectiveExtension
call with iterating the matches for name (from result or the function that
returns all matches), select the top-precedence match with entry.Enabled !=
enable (prefer workspace/user overrides over bundled), and if found call
toggleEntry with that entry; if none found return the existing "not
found/already in desired state" error.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d791b4d1-a09d-47ff-aca9-2faf6e21ecb7 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes:
  - The current toggle path uses `findEffectiveExtension(result, name)`, which always selects a single highest-precedence declaration and never examines lower-precedence duplicates whose enabled state may still differ from the requested action.
  - Root cause: enable/disable target selection is based on the collapsed effective view instead of the full discovered-match set.
  - Implemented fix: chose the highest-precedence discovered entry for the requested name whose `Enabled` state differs from the requested toggle value, then passed that entry to `toggleEntry`.
