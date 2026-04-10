---
status: pending
title: ACP MCP plumbing and nested `run_agent` execution engine
type: backend
complexity: critical
dependencies:
  - task_01
  - task_02
---

# Task 03: ACP MCP plumbing and nested `run_agent` execution engine

## Overview
Extend the ACP execution path so sessions can receive a merged MCP server set composed of agent-local MCP servers plus the reserved Compozy MCP server, and implement the nested execution engine behind the generic `run_agent` tool. This task is the architectural core of the feature because it bridges static agent definitions into live ACP sessions and makes child-agent execution available inside ACP-backed runs.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists for this feature. Technical requirements are derived from `_techspec.md` and the accepted ADRs.
</critical>

<requirements>
- MUST extend the ACP session request path so resolved MCP servers can be attached on both `session/new` and `session/load`.
- MUST merge the reserved Compozy MCP server with the selected agent's `mcp.json` MCP servers using the precedence and reserved-name rules from the TechSpec.
- MUST implement the reserved `compozy` MCP server behavior behind a reusable internal engine that can serve the generic `run_agent` tool.
- MUST ensure `run_agent` creates real nested ACP sessions instead of inline function-style execution so child-agent runtime defaults are honored.
- MUST keep nested depth as host-owned state, not as a caller-controlled tool argument, and enforce an internal maximum depth safeguard.
- MUST ensure child runs do not inherit parent agent-local MCP servers implicitly; each child run receives only the reserved `compozy` MCP server plus the child agent's own `mcp.json`.
- MUST prevent nested access-mode escalation by capping the child effective access mode at the parent effective access mode.
- MUST return structured success and failure payloads from `run_agent` so ACP callers receive deterministic results.
</requirements>

## Subtasks
- [ ] 03.1 Extend ACP client session request models to carry resolved MCP server definitions for both new-session and load-session flows.
- [ ] 03.2 Add MCP merge logic that combines the reserved `compozy` server with agent-local MCP servers while enforcing reserved-name protection and child-run isolation.
- [ ] 03.3 Implement the reusable nested execution engine that resolves a child agent, computes host-owned nested context, and launches a real child ACP session.
- [ ] 03.4 Implement the generic `run_agent` tool contract and structured result shape on top of the nested execution engine.
- [ ] 03.5 Enforce core nested-execution safeguards for max depth, access-mode ceiling, invalid child references, and deterministic failure reporting.
- [ ] 03.6 Add unit and integration coverage for merged MCP session setup, nested child execution, and safeguarded failure paths.

## Implementation Details
See TechSpec "Data Flow", "Core Interfaces", and "NestedExecutionContext" for the host-owned execution model, and ADR-003 for the rule that nested execution must flow through the reserved Compozy MCP capability rather than runtime-specific bespoke wiring.

Keep this task focused on execution semantics and ACP plumbing. Do not wire Cobra commands here beyond what is minimally necessary for an internal executable surface; the public CLI exposure belongs to the next task. The result of this task should be an internal engine that the CLI can invoke, not a CLI-first implementation.

### Relevant Files
- `internal/core/agent/client.go` — Current ACP `CreateSession` and `ResumeSession` paths that still hardcode empty `McpServers`.
- `internal/core/agent/client_test.go` — Existing ACP client coverage to extend with MCP server assertions.
- `internal/core/run/internal/acpshared/command_io.go` — Shared session creation path that must pass the merged MCP server set.
- `internal/core/run/internal/acpshared/command_io_test.go` — Existing shared-session tests to extend with agent-backed MCP setup expectations.
- `internal/core/agent/session.go` — Existing ACP session abstraction that nested runs may need to coordinate with.
- `internal/core/agents/` — Registry and runtime-resolution output consumed to launch child sessions and merge MCP config.
- `internal/core/agents/mcpserver/` — New internal package or package slice to create for the reserved Compozy MCP server engine.

### Dependent Files
- `internal/cli/root.go` — The hidden `mcp-serve` entrypoint added later will invoke the internal engine built in this task.
- `internal/cli/commands.go` — `exec --agent` will later depend on the merged MCP and nested-execution behavior established here.
- `internal/core/run/executor/execution_acp_integration_test.go` — End-to-end ACP behavior will need to cover merged MCP setup and nested execution.

### Related ADRs
- [ADR-003: Expose Nested Agent Execution Through a Compozy-Owned MCP Server](adrs/adr-003.md) — Primary ADR for reserved MCP server semantics and nested execution.
- [ADR-005: Use Agent-Local `mcp.json` with Standard MCP Configuration Shape](adrs/adr-005.md) — Governs MCP merge behavior and reserved-name protection.
- [ADR-004: Keep V1 Agent Capability Scope Minimal](adrs/adr-004.md) — Confines the MCP server to the single generic `run_agent` tool.

## Deliverables
- ACP session request support for resolved MCP server definitions on create and resume.
- Internal MCP merge logic for reserved and agent-local MCP servers.
- Internal nested execution engine backing the generic `run_agent` tool.
- Structured `run_agent` request/result contract with deterministic failure reporting.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for merged MCP session setup and nested execution **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] `CreateSession` forwards the merged MCP server list into `acp.NewSessionRequest`.
  - [ ] `ResumeSession` forwards the merged MCP server list into `acp.LoadSessionRequest`.
  - [ ] Merging agent-local MCP servers with the reserved `compozy` server rejects name collisions before session start.
  - [ ] A child run receives its own agent-local MCP servers and does not inherit unrelated parent agent-local MCP servers.
  - [ ] A child whose requested access mode is broader than the parent effective mode is capped instead of elevated.
  - [ ] A missing child agent name returns a structured `run_agent` failure payload rather than a transport crash.
- Integration tests:
  - [ ] A parent ACP-backed run can invoke `run_agent` and receive a successful structured response from a real nested child session.
  - [ ] A resumed ACP session still reattaches the merged MCP server set before the next prompt turn.
  - [ ] A nested call blocked by max depth surfaces a deterministic failure result without corrupting the parent session.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Agent-backed ACP sessions attach the correct MCP servers on both create and resume paths.
- `run_agent` launches real child ACP sessions with their own resolved runtime and MCP configuration.
- Nested runs cannot escalate access or exceed internal depth safeguards.
