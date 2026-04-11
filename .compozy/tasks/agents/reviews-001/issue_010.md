---
status: resolved
file: internal/core/agents/mcpserver/server.go
line: 129
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5tw,comment:PRRC_kwDORy7nkc62zc8f
---

# Issue 010: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Fail closed when the reserved host context is missing.**

If `COMPOZY_RUN_AGENT_CONTEXT` is unset, this falls back to a zero-value `HostContext` and still serves `run_agent`. That turns a parent/child contract failure into fail-open behavior: nested depth, agent path, workspace root, and inherited access constraints can all be lost.

<details>
<summary>Suggested fix</summary>

```diff
 	raw, ok := lookupEnv(reusableagents.RunAgentContextEnvVar)
 	if !ok || strings.TrimSpace(raw) == "" {
-		return HostContext{}, nil
+		return HostContext{}, fmt.Errorf("missing %s", reusableagents.RunAgentContextEnvVar)
 	}
```
</details>

Tests can still exercise the server by calling `RunStdio` with an explicit `HostContext` instead of relying on the env loader.


Based on learnings "Assess and document attack surface changes for every architectural decision".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agents/mcpserver/server.go` around lines 121 - 129, The loader
function loadHostContextFromEnv currently returns a zero HostContext when the
reserved env var reusableagents.RunAgentContextEnvVar is missing, causing
fail-open behavior; change loadHostContextFromEnv so that if lookupEnv reports
the var absent or blank it returns a non-nil error (with a clear message
referencing the missing COMPOZY_RUN_AGENT_CONTEXT) instead of HostContext{}, and
ensure callers (e.g., server startup that may call RunStdio) propagate or handle
that error (tests should call RunStdio with an explicit HostContext to continue
exercising the server).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d740b4bc-0bac-4faf-9dba-d2618b9a24f6 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `loadHostContextFromEnv()` returned a zero-value host context when `COMPOZY_RUN_AGENT_CONTEXT` was missing, which failed open and dropped nested execution constraints.
- Fix: Changed the loader to fail closed with a missing-env error and updated CLI/server tests to pass explicit reserved host context where the server is intentionally exercised.
- Evidence: `go test ./internal/core/agents/... ./internal/cli`
