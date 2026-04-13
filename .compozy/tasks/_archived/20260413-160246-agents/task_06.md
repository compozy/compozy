---
status: completed
title: User-facing documentation and example agent fixtures
type: docs
complexity: medium
dependencies:
  - task_04
  - task_05
---

# Task 06: User-facing documentation and example agent fixtures

## Overview
Document how reusable agents work in Compozy and provide concrete example agent fixtures that users can copy and adapt. This task makes the feature adoptable by explaining the filesystem layout, CLI commands, MCP behavior, and the boundary between agent-local MCP config and the reserved Compozy MCP server.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists for this feature. Technical requirements are derived from `_techspec.md` and the accepted ADRs.
</critical>

<requirements>
- MUST document both supported discovery scopes: workspace `.compozy/agents/<name>/` and global `~/.compozy/agents/<name>/`.
- MUST document the `AGENT.md` frontmatter-plus-body format, allowed metadata fields, naming rules, and reserved-name restriction.
- MUST document the optional `mcp.json` shape, placeholder-expansion behavior, fail-closed missing-env behavior, and the difference between agent-local MCP servers and the reserved Compozy MCP server.
- MUST document `compozy exec --agent`, `compozy agents list`, and `compozy agents inspect` with examples aligned to the implemented CLI.
- MUST include at least one concrete example agent fixture showing a valid `AGENT.md` and one example `mcp.json`.
- MUST keep docs consistent with the actual implementation and tests, not with outdated intermediate design ideas.
</requirements>

## Subtasks
- [x] 06.1 Update the main user-facing docs to introduce reusable agents and show the supported command surface.
- [x] 06.2 Add a focused documentation page or section for agent directory structure, `AGENT.md`, and `mcp.json`.
- [x] 06.3 Add example fixtures or examples that demonstrate a minimal agent and an agent with external MCP dependencies.
- [x] 06.4 Verify the docs against implemented CLI behavior, validation errors, and nested-execution semantics before finalizing.

## Implementation Details
See TechSpec "Executive Summary", "Data Models", and "Runtime precedence" for the language that the docs must stay aligned with. The documentation should clearly separate two concepts that were easy to confuse during design: agent-local MCP servers declared in `mcp.json`, and the reserved Compozy MCP server that exists only to expose host-owned tools such as `run_agent`.

Prefer updating existing top-level user docs before creating deep new documentation trees. If a dedicated page is needed, keep it small and linked from `README.md`.

### Relevant Files
- `README.md` — Main user-facing command and configuration documentation that should introduce reusable agents.
- `docs/` — Existing documentation area for any focused follow-up page such as agent structure or runtime behavior.
- `.compozy/tasks/agents/_techspec.md` — Source of truth for user-facing behavior that docs must stay aligned with.
- `.compozy/tasks/agents/adrs/` — Source of truth for design decisions that the docs may summarize without duplicating.
- `.compozy/agents/` — Example fixture location or example structure to document, if sample artifacts are committed.

### Dependent Files
- `internal/cli/root.go` — The implemented CLI commands must remain in sync with the docs written here.
- `internal/core/agents/` — Validation behavior and file-shape rules documented here are enforced by this package.

### Related ADRs
- [ADR-001: Flat Filesystem Agent Definitions with Workspace Override](adrs/adr-001.md) — Governs filesystem layout and override semantics.
- [ADR-002: Assemble Agent System Prompts from Resolved Metadata with Explicit Override Precedence](adrs/adr-002.md) — Governs execution precedence behavior that docs should explain accurately.
- [ADR-003: Expose Nested Agent Execution Through a Compozy-Owned MCP Server](adrs/adr-003.md) — Governs the distinction between reserved host-owned MCP and agent-local MCP.
- [ADR-005: Use Agent-Local `mcp.json` with Standard MCP Configuration Shape](adrs/adr-005.md) — Governs example `mcp.json` structure and environment-placeholder rules.

## Deliverables
- Updated user-facing docs for reusable agents and the new CLI surface.
- Example `AGENT.md` and `mcp.json` fixtures or examples aligned with the implementation.
- Documentation review against implemented behavior and validation rules.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for documented examples and CLI snippets **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Any doc-based example fixtures parse successfully as valid agent definitions.
  - [x] Example `mcp.json` snippets used in docs satisfy the same validation rules as real agent config.
- Integration tests:
  - [x] The documented `compozy exec --agent <name>` example works against a real example fixture.
  - [x] The documented `compozy agents inspect <name>` example matches the implemented command output shape closely enough to stay trustworthy.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- A user can create and run a reusable agent from the docs without needing to read the TechSpec.
- The docs clearly distinguish agent-local MCP configuration from the reserved Compozy MCP tool server.
- The examples stay synchronized with the implemented CLI and validation behavior.
