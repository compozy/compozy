---
status: pending
title: Observability, safeguards, and end-to-end integration hardening
type: test
complexity: high
dependencies:
  - task_03
  - task_04
---

# Task 05: Observability, safeguards, and end-to-end integration hardening

## Overview
Harden the reusable-agent feature by making nested execution observable, blocking failure modes explicitly, and covering the cross-package flows with end-to-end tests. This task turns the first working implementation into an operationally trustworthy feature by ensuring users and maintainers can diagnose blocked or failed runs without guessing.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists for this feature. Technical requirements are derived from `_techspec.md` and the accepted ADRs.
</critical>

<requirements>
- MUST emit structured runtime signals for the major agent lifecycle events introduced by this feature, including resolution, prompt assembly, MCP merge, nested start, nested completion, and nested blocking.
- MUST classify blocked nested runs with explicit machine-readable reasons such as depth-limit, cycle-detected, access-denied, invalid-agent, or invalid-mcp.
- MUST cover resumed-session behavior so agent-backed resumed runs reattach MCP servers and preserve deterministic nested safeguards.
- MUST add end-to-end tests for nested execution cycles, blocked depth growth, missing env vars in `mcp.json`, workspace override behavior, and successful parent-child execution.
- MUST keep failure handling deterministic: blocked or invalid runs must return structured failures, not panics, broken streams, or partial session corruption.
- MUST ensure the feature remains compatible with existing execution logging and event-journal patterns already used by Compozy.
</requirements>

## Subtasks
- [ ] 05.1 Add structured logging or event emission for agent resolution, prompt assembly, MCP merge, nested execution start/completion, and blocked nested runs.
- [ ] 05.2 Introduce a stable blocked-reason vocabulary and use it consistently across runtime logs, tool failures, and CLI-facing error translation.
- [ ] 05.3 Add integration fixtures for cyclic agent calls, missing `mcp.json` environment variables, and workspace-overrides-global scenarios.
- [ ] 05.4 Extend end-to-end ACP and CLI tests to cover success, resume, block, and validation-failure flows.
- [ ] 05.5 Run the full verification pipeline and tighten any weak error handling, flaky assumptions, or missing instrumentation revealed by the new tests.

## Implementation Details
See TechSpec "Operational Considerations", "Error Handling", and the review-driven notes captured in the ledger for the expected blocked-reason behavior and resumed-session MCP reattachment guarantees.

Do not treat this as a pure test-only task. If the new end-to-end coverage reveals missing runtime events, weak structured errors, or inconsistent blocked-reason handling, fix the production code instead of weakening the tests.

### Relevant Files
- `internal/core/run/internal/acpshared/session_handler.go` — Existing runtime event flow that agent-specific observability should align with.
- `internal/core/run/internal/acpshared/session_handler_test.go` — Existing runtime-event coverage to extend.
- `internal/core/run/internal/acpshared/command_io_test.go` — Shared execution tests to extend with agent-backed instrumentation expectations.
- `internal/core/agent/client_test.go` — ACP client coverage that should assert resumed-session MCP behavior.
- `internal/core/run/executor/execution_acp_integration_test.go` — Main end-to-end ACP integration test surface to extend.
- `internal/cli/root_command_execution_test.go` — CLI-level integration tests for agent execution and validation flows.

### Dependent Files
- `README.md` — Later documentation should describe the stable failure modes and diagnostics produced here.
- `internal/cli/root.go` — CLI error presentation may need to reflect the blocked-reason vocabulary established in this task.
- `internal/core/agents/` — Validation and resolution errors may need stronger structured forms based on hardening findings.

### Related ADRs
- [ADR-003: Expose Nested Agent Execution Through a Compozy-Owned MCP Server](adrs/adr-003.md) — Governs nested execution behavior and failure handling.
- [ADR-005: Use Agent-Local `mcp.json` with Standard MCP Configuration Shape](adrs/adr-005.md) — Governs missing-env validation and MCP diagnostics.

## Deliverables
- Structured observability for the reusable-agent lifecycle.
- Stable blocked-reason classification for nested-run failures.
- Expanded end-to-end test coverage for success, failure, resume, and override scenarios.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for full agent execution flows **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] A blocked nested run due to max depth emits the expected blocked-reason value.
  - [ ] A cycle such as `A -> B -> A` is classified as cycle-detected instead of appearing as a generic internal error.
  - [ ] A missing environment variable referenced by `mcp.json` surfaces as an invalid-MCP-style failure with actionable details.
  - [ ] Resumed agent-backed sessions emit the same MCP merge observability as fresh sessions.
- Integration tests:
  - [ ] A successful parent-child run emits the expected resolution, prompt-assembly, MCP-merge, nested-start, and nested-complete events.
  - [ ] A resumed persisted exec run with `--agent` reattaches MCP servers before the next nested call.
  - [ ] Workspace override of a global agent is honored consistently in CLI execution and nested tool invocation.
  - [ ] A cyclic nested call sequence is blocked without leaving the parent session unusable.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Reusable-agent failures are diagnosable from logs, structured results, and CLI output without guessing.
- Cycles, depth overflow, invalid agents, and invalid MCP configs are blocked deterministically.
- The feature has end-to-end coverage for fresh and resumed sessions across the main success and failure paths.
