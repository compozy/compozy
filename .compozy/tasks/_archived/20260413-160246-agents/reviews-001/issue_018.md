---
status: resolved
file: internal/setup/install_test.go
line: 207
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5t-,comment:PRRC_kwDORy7nkc62zc8v
---

# Issue 018: _🛠️ Refactor suggestion_ | _🟠 Major_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_

**Derive the expected reusable-agent roster instead of pinning it to `6`, and fold these into a table-driven test.**

The `== 6` assertions will break whenever a bundled reusable agent is added or removed, even if install behavior is still correct. These two cases also share the same setup/assertion shape, so this is a good spot to drive expectations from the embedded bundle and cover preview/install via `t.Run(...)`. As per coding guidelines: `**/*_test.go`: Use table-driven tests with subtests (\`t.Run\`) as the default test pattern.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/setup/install_test.go` around lines 145 - 207, Both tests hardcode
the expected number of bundled reusable agents (6) and duplicate setup/assertion
logic; change them to a single table-driven test with two subtests ("preview"
and "install") that share setup and derive the expected roster dynamically from
the source of truth for bundled agents (e.g., call whichever function or
variable your package exposes that lists bundled reusable agents — reference
PreviewBundledReusableAgentInstall and InstallBundledReusableAgents to run the
respective flows), assert len(results) == len(bundledRoster) rather than == 6,
and keep the existing per-item path assertions (TargetPath and success.Path)
inside the subtests. Ensure you replace the two test functions
TestPreviewBundledReusableAgentInstallUsesGlobalCompozyAgentsDir and
TestInstallBundledReusableAgentsCopiesCouncilRosterIntoGlobalCompozyAgentsDir
with the consolidated table-driven t.Run structure and derive expected counts
from the bundled roster.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5b83f2d8-737c-414c-9d4a-933187b6f725 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: The reusable-agent install tests hardcoded the bundled roster size and duplicated preview/install assertions, making the tests brittle to legitimate roster changes.
- Fix: Consolidated the preview/install cases into a table-driven test and derived expected counts from `ListBundledReusableAgents()`.
- Evidence: `go test ./internal/setup`
