# Workflow Memory

Keep only durable, cross-task context here. Do not duplicate facts that are obvious from the repository, PRD documents, or git history.

## Current State

## Shared Decisions
- `internal/core/agents` discovery returns both resolved agents and per-agent validation problems so malformed agent directories do not block later tasks from listing or resolving other valid agents.
- Agent-backed runtime precedence is now enforced by carrying explicit runtime-flag state in `model.RuntimeConfig` and `core.Config`; reusable-agent defaults only override runtime fields that were not explicitly set by the caller.
- `internal/core/agents.ResolveExecutionContext` is the shared task-02 seam for agent-backed execution. It resolves the selected agent once, mutates the effective runtime in place, and assembles the canonical system prompt as `base framing -> <agent_metadata> -> <available_agents> -> agent body`.
- Agent-backed ACP sessions now carry resolved MCP servers as runtime data on prepared jobs and exec jobs, so both `session/new` and `session/load` reattach the same merged server set.
- The reserved `compozy` MCP server context is serialized in `COMPOZY_RUN_AGENT_CONTEXT` as JSON containing `NestedBaseRuntime` plus `NestedExecutionContext`; child calls inherit host-owned depth, max-depth, parent run, and parent access state instead of caller-provided values.
- The public reusable-agent CLI contract is now `compozy exec --agent <name>`, `compozy agents list`, and `compozy agents inspect`; the reserved MCP host stays internal as hidden `compozy mcp-serve --server compozy` so root help remains operator-focused.

## Shared Learnings
- Exec mode bypasses `internal/core/plan.Prepare`, so any future child-agent or `exec --agent` work must reuse `ResolveExecutionContext` from both the workflow-preparation path and `internal/core/run/exec/exec.go` instead of assuming planning is the only integration point.
- Real nested child-agent execution now flows through `exec.ExecutePreparedPrompt`, which opens a genuine ACP-backed exec turn while suppressing nested stdout emission in favor of the structured `run_agent` result payload.

## Open Risks

## Handoffs
