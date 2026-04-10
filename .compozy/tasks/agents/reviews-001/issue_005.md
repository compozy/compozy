---
status: resolved
file: internal/core/agent/client.go
line: 789
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5to,comment:PRRC_kwDORy7nkc62zc8T
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Fail fast on unsupported MCP transports.**

`toACPMCPServers` silently drops every entry whose `Stdio` config is nil. After this change, create/resume can succeed with a partial MCP set instead of surfacing an invalid session configuration, which is a hard-to-debug failure mode for nested agents and forwarded session servers. Return an error here, or validate earlier, when a requested server cannot be represented in ACP.


Based on learnings: Prioritize system boundaries and ownership - ensure clear ownership and API contracts between system components; Name concrete failure modes when identifying potential issues.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/client.go` around lines 765 - 789, toACPMCPServers
currently drops MCPServer entries with nil Stdio, allowing invalid/partial MCP
configurations to proceed; change it to fail fast by returning an error when any
src[i].Stdio == nil (include identifying info such as index or server name from
model.MCPServer in the error message). Update the function signature from
toACPMCPServers(src []model.MCPServer) []acp.McpServer to return
([]acp.McpServer, error) (or perform equivalent upstream validation before
calling it), and ensure callers handle and propagate the error so create/resume
rejects unsupported transports instead of silently omitting them.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:44db1207-e0c3-4af8-b043-4fce2c12b432 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `toACPMCPServers()` silently dropped `model.MCPServer` entries with `Stdio == nil`, allowing invalid partial MCP configuration to proceed into session setup.
- Fix: Changed MCP conversion to return an error for unsupported transports and made both `CreateSession()` and `ResumeSession()` reject invalid ACP MCP configuration before issuing the session request.
- Evidence: `go test ./internal/core/agent`
