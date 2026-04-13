---
status: resolved
file: internal/core/agents/mcpserver/server_test.go
line: 99
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56QRk6,comment:PRRC_kwDORy7nkc62z8SX
---

# Issue 003: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Make this failure-path test hermetic.**

With `NewServer()` and `HostContext{}`, the default engine can still resolve agents from the runner’s real home/workspace. If a real `missing-agent` exists, this assertion stops exercising the invalid-agent path. Point `BaseRuntime.WorkspaceRoot` at `t.TempDir()` and override `HOME`/`XDG_CONFIG_HOME` to temp dirs before invoking the handler.



Based on learnings "Find and document edge cases that the happy path ignores".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agents/mcpserver/server_test.go` around lines 78 - 99, The test
uses NewServer() and HostContext{} which can pick up real-agent files; make the
failure-path hermetic by creating a temp workspace and config dirs and wiring
them into the server/runtime before invoking handler: set the server's
BaseRuntime.WorkspaceRoot to t.TempDir() (or otherwise point BaseRuntime on the
created temp dir) and set the environment variables HOME and XDG_CONFIG_HOME to
temp dirs (via t.TempDir()) before calling server.runAgentTool(HostContext{})
and executing handler(context.Background(), ...), ensuring the real runner
workspace/config cannot resolve a "missing-agent".
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e61192d9-7c66-438c-8efd-0a27424736ab -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `TestRunAgentToolMarksStructuredFailuresAsToolErrors` currently uses `NewServer()` with an empty `HostContext`, so nested agent resolution can still consult the runner's real home directory and discover globally installed agents.
  - That means a real `missing-agent` install could cause this test to exercise the wrong path and stop proving the invalid-agent failure case.
  - Intended fix: pin `HOME`, `XDG_CONFIG_HOME`, and `BaseRuntime.WorkspaceRoot` to temp directories before invoking the handler so the missing-agent path is fully hermetic.
  - Resolution: updated `internal/core/agents/mcpserver/server_test.go` to run the failure-path test against temp `HOME` / `XDG_CONFIG_HOME` directories and an isolated workspace root, removing the dependency on any real globally installed agents.
  - Verification: `go test ./internal/core/agents/... ./internal/core/run/exec/...` and `make verify`.
