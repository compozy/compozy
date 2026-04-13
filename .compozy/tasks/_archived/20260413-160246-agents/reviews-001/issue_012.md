---
status: resolved
file: internal/core/run/exec/prompt_exec.go
line: 60
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5t1,comment:PRRC_kwDORy7nkc62zc8l
---

# Issue 012: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Fail the run through the normal completion path when MCP setup errors.**

`writeStarted` has already persisted/journaled the exec turn before `buildMCPServers` runs. If the builder fails here, the function returns immediately and never calls `completeTurn`, which can leave a started run hanging for MCP-resolution failures. Route this branch through the same terminal-failure path used for execution errors.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/exec/prompt_exec.go` around lines 51 - 60, After
writeStarted succeeds but buildMCPServers fails, route the error through the
same terminal-completion path by invoking completeTurn (using state and cfg)
with a failure result before returning; specifically, in the buildMCPServers
error branch (the call to buildMCPServers(state.runArtifacts.RunID)), call
completeTurn to mark the run as completed/failed and persist the terminal state,
then return PreparedPromptResult{} with the original error — ensuring the branch
mirrors the execution-error handling rather than returning early and leaving a
started run hanging.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:ee6f376d-2c51-442f-8f6e-f006907140c7 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `ExecutePreparedPrompt()` called `writeStarted()` before the MCP builder and then returned immediately on builder failure, leaving the nested exec run without a terminal failed completion record.
- Fix: Routed MCP-builder failures through `completeTurn()` with failed status so the run record, artifacts, and terminal state are persisted before the error is returned.
- Evidence: `go test ./internal/core/run/exec`
