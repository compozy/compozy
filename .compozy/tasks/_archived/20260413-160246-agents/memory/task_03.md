# Task Memory: task_03.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement task 03 end to end: ACP create/load session plumbing must attach merged MCP servers, and the reserved `compozy` capability must expose a reusable internal `run_agent` engine that launches real child ACP sessions with structured results and host-owned safeguards.

## Important Decisions
- Use the approved TechSpec as the design artifact for this task, so implementation can proceed without a new design round.
- Reuse `internal/core/agents.ResolveExecutionContext` and the existing resolved `MCPConfig` surface rather than rebuilding agent/runtime precedence or `mcp.json` parsing in ACP-specific code.
- Keep task 03 scoped to the internal nested execution engine; the hidden `compozy mcp-serve` CLI command remains task 04 work.
- Model merged MCP servers as first-class runtime data on prepared jobs so both new-session and load-session ACP flows can reattach the same resolved server set.
- Serialize reserved-server host state as `COMPOZY_RUN_AGENT_CONTEXT` JSON carrying `NestedBaseRuntime` plus `NestedExecutionContext`; nested tool callers do not control depth or inherited runtime.
- Back the generic `run_agent` contract with `exec.ExecutePreparedPrompt`, so child agents run through the real ACP exec path and honor their own runtime defaults.

## Learnings
- The ACP SDK already supports `mcpServers` on both `NewSessionRequest` and `LoadSessionRequest`; current Compozy code simply leaves both lists empty.
- The existing exec path captures real ACP session output and runtime defaults, but it is persistence-oriented, so task 03 likely needs a narrower reusable engine layer for nested runs.
- The ACP SDK validates `mcpServers` as a required field, so the client conversion layer must emit an empty slice rather than `nil` when no MCP servers are attached.
- Task-owned file coverage reached 90.2% across `internal/core/agents/execution.go`, `internal/core/agents/session_mcp.go`, `internal/core/agents/mcpserver/engine.go`, and `internal/core/run/exec/prompt_exec.go`.

## Files / Surfaces
- `internal/core/agent/{client.go,client_test.go}`
- `internal/core/agents/{agents.go,execution.go,execution_test.go,session_mcp.go,session_mcp_test.go}`
- `internal/core/agents/mcpserver/{engine.go,engine_test.go}`
- `internal/core/model/mcp.go`
- `internal/core/plan/prepare.go`
- `internal/core/run/internal/acpshared/{command_io.go,command_io_test.go,session_exec.go}`
- `internal/core/run/internal/runshared/config.go`
- `internal/core/run/exec/{exec.go,exec_test.go,exec_integration_test.go,prompt_exec.go,run_agent_engine_integration_test.go,test_helpers_test.go}`

## Errors / Corrections
- Fixed a test harness issue where a prompt-exec test used `t.Setenv` under `t.Parallel`; the test now runs non-parallel.

## Ready for Next Run
- Task 03 implementation is complete and verified with `make verify`.
