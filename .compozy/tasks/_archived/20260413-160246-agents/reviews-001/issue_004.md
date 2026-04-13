---
status: resolved
file: internal/cli/reusable_agents_doc_examples_test.go
line: 20
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5th,comment:PRRC_kwDORy7nkc62zc8M
---

# Issue 004: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Isolate global agent discovery in these CLI fixture tests.**

Both tests rely on `reviewer` resolving from the copied workspace fixture, but they leave `HOME` and `XDG_CONFIG_HOME` pointing at the real machine state. A globally installed `reviewer` can change the resolved source or output and make these tests flaky outside a clean CI box. Set both env vars to temp dirs before invoking the CLI.


Based on learnings: Find and document edge cases that the happy path ignores.


Also applies to: 44-47

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/reusable_agents_doc_examples_test.go` around lines 17 - 20, Set
HOME and XDG_CONFIG_HOME to isolated temp dirs before running the CLI fixture so
global agent discovery cannot influence resolution; specifically, in the test(s)
that create workspaceRoot and call writeCLIWorkspaceConfig,
copyCLIDocumentedAgentFixture("reviewer") and withWorkingDir, create two temp
dirs, set os.Setenv("HOME", tmpHome) and os.Setenv("XDG_CONFIG_HOME", tmpXdg)
before invoking the CLI actions, and restore the original env values (defer
reset) after the test; apply the same change to the other similar test that uses
the same sequence of
workspaceRoot/writeCLIWorkspaceConfig/copyCLIDocumentedAgentFixture/withWorkingDir
calls.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:44db1207-e0c3-4af8-b043-4fce2c12b432 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: The documented CLI fixture tests isolated the workspace cwd but still inherited the real `HOME` / `XDG_CONFIG_HOME`, so globally installed agents could affect discovery and output.
- Fix: Set both `HOME` and `XDG_CONFIG_HOME` to temp directories before invoking the documented `agents inspect` and `exec --agent` examples.
- Evidence: `go test ./internal/cli`
