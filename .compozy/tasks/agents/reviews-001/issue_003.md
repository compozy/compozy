---
status: resolved
file: internal/cli/agents_commands_test.go
line: 30
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5tZ,comment:PRRC_kwDORy7nkc62zc8B
---

# Issue 003: _⚠️ Potential issue_ | _🔴 Critical_
## Review Comment

_⚠️ Potential issue_ | _🔴 Critical_

**These tests race on the process working directory.**

`withWorkingDir(...)` almost certainly uses `os.Chdir`, which is process-global, but several of these tests are marked `t.Parallel()`. That makes command resolution and persisted-run assertions flaky across unrelated tests in the same package. Either remove `t.Parallel()` from cwd-mutating tests or stop relying on process-wide cwd in the helper.  



Also applies to: 67-69, 249-275, 300-324

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/agents_commands_test.go` around lines 24 - 30, The tests calling
withWorkingDir (e.g., TestExecCommandPassesSelectedAgentIntoWorkflowConfig)
mutate the process-wide working directory while marked t.Parallel(), causing
racey failures; fix by either removing t.Parallel() from any test that calls
withWorkingDir (and the other affected tests at the indicated ranges) or,
better, change the helper and command invocations to avoid os.Chdir: update
withWorkingDir and any command execution to set the working directory locally
(use exec.Command.Dir or equivalent) so tests can remain parallel-safe and no
longer rely on process-wide cwd.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:ee6f376d-2c51-442f-8f6e-f006907140c7 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Analysis: The cited tests already serialize process-wide cwd mutation through `withWorkingDir()`, which acquires `cliWorkingDirMu`, changes cwd, and only releases the mutex after cleanup restores the original cwd.
- Why no change: Within the `internal/cli` test binary this prevents concurrent cwd mutation, so the reported race condition does not reproduce against the current helper.
- Evidence: inspected `withWorkingDir()` in `internal/cli/root_command_execution_test.go` and reran `go test ./internal/cli`
