---
status: completed
title: Agent registry, parsing, validation, and override resolution
type: backend
complexity: high
dependencies: []
---

# Task 01: Agent registry, parsing, validation, and override resolution

## Overview
Create the reusable agent definition core under `internal/core/agents` so Compozy can discover agents from workspace and global scopes, parse `AGENT.md`, load optional `mcp.json`, and resolve a single effective agent view. This task establishes the canonical data model and validation rules that every later CLI, ACP, and nested-execution path will depend on.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists for this feature. Technical requirements are derived from `_techspec.md` and the accepted ADRs.
</critical>

<requirements>
- MUST create a new agent-definition package under `internal/core/agents` that owns discovery, parsing, validation, and resolution of reusable agents.
- MUST support both `.compozy/agents/<name>/AGENT.md` and `~/.compozy/agents/<name>/AGENT.md` discovery scopes, with workspace scope overriding global scope for the entire agent directory.
- MUST parse `AGENT.md` as YAML frontmatter plus markdown body using the repository's existing frontmatter conventions.
- MUST validate agent directory names against `^[a-z][a-z0-9-]{0,63}$` and reject the reserved name `compozy`.
- MUST support only the v1 metadata fields defined in the TechSpec and reject unsupported inheritance/composition-era fields such as `extends`, `uses`, `skills`, and `memory`.
- MUST load optional `mcp.json` using the standard `mcpServers` object shape, expand environment placeholders during load/setup, and fail closed when a referenced environment variable is missing.
- MUST ensure the reserved MCP server name `compozy` cannot be declared or overridden from `mcp.json`.
- MUST keep discovery and validation independent from ACP transport code so later tasks can consume a resolved agent model without re-parsing files.
</requirements>

## Subtasks
- [x] 01.1 Create the `internal/core/agents` package with models for resolved agents, metadata, source scope, and optional agent-local MCP configuration.
- [x] 01.2 Implement filesystem discovery for workspace and global agent roots, including whole-directory workspace override semantics when agent names collide.
- [x] 01.3 Implement `AGENT.md` parsing with YAML frontmatter plus markdown body and validate the supported v1 metadata fields.
- [x] 01.4 Implement `mcp.json` loading, placeholder expansion, reserved-name protection, and fail-closed validation for missing environment variables.
- [x] 01.5 Add focused validation errors for invalid slugs, reserved names, malformed frontmatter, malformed MCP config, and unsupported deferred features.
- [x] 01.6 Add unit tests covering successful discovery, scope override, and validation failures for the edge cases above.

## Implementation Details
See TechSpec "System Architecture", "Implementation Design", and "Data Models" for the resolved agent contract, filesystem layout, validation rules, and MCP config shape.

This task should only establish agent-definition semantics. It must not couple the package to Cobra commands, ACP connection setup, or nested execution flow. Those integrations belong to later tasks and should consume the new registry rather than reimplement discovery logic.

### Relevant Files
- `internal/core/frontmatter/frontmatter.go` — Existing YAML frontmatter parser to reuse for `AGENT.md`.
- `internal/core/frontmatter/frontmatter_test.go` — Existing parser expectations that should remain compatible.
- `internal/core/workspace/config.go` — Existing workspace root resolution that agent discovery should align with.
- `internal/core/workspace/config_types.go` — Confirms agents are not added to `.compozy/config.toml` in v1.
- `internal/core/model/workspace_paths.go` — Existing workspace-relative path conventions to mirror for agent roots.
- `internal/core/agents/` — New package to create for registry, parsing, validation, and source resolution.

### Dependent Files
- `internal/core/run/internal/acpshared/command_io.go` — Will later consume resolved agent data instead of raw prompt/config fragments.
- `internal/cli/commands.go` — `exec --agent` will depend on this registry to resolve selected agents.
- `internal/cli/root.go` — `compozy agents` command group will later depend on discovery output from this task.

### Related ADRs
- [ADR-001: Flat Filesystem Agent Definitions with Workspace Override](adrs/adr-001.md) — Governs discovery shape, naming rules, and workspace precedence.
- [ADR-004: Keep V1 Agent Capability Scope Minimal](adrs/adr-004.md) — Constrains the parser to the reduced v1 capability set.
- [ADR-005: Use Agent-Local `mcp.json` with Standard MCP Configuration Shape](adrs/adr-005.md) — Governs `mcp.json` semantics and reserved-name handling.

## Deliverables
- New `internal/core/agents` package for discovery, parsing, validation, and resolution.
- Resolved agent model covering metadata, prompt body, source scope, and optional MCP config.
- Validation errors for invalid names, unsupported fields, malformed frontmatter, malformed MCP config, missing env vars, and reserved-name conflicts.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for workspace/global resolution behavior **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Parsing a valid `AGENT.md` returns the expected metadata values and markdown body.
  - [x] An agent directory named `compozy` is rejected with a reserved-name validation error.
  - [x] An agent directory whose name contains uppercase letters or invalid punctuation is rejected with a slug validation error.
  - [x] `mcp.json` placeholder expansion fails with a descriptive validation error when a referenced environment variable is absent.
  - [x] `mcp.json` that declares an MCP server named `compozy` is rejected before session setup.
  - [x] Unsupported deferred fields such as `skills` or `memory` are rejected instead of silently ignored.
- Integration tests:
  - [x] When the same agent exists in global and workspace scopes, the workspace directory wins as a whole.
  - [x] Discovery returns both scopes correctly when no collisions exist.
  - [x] A malformed agent in one directory surfaces validation failure without corrupting resolution of other valid agents.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Compozy can discover and resolve agents from both supported scopes without any CLI-specific code.
- `AGENT.md` and `mcp.json` validation rules match the TechSpec and accepted ADRs.
- The resulting registry surface is stable enough for later prompt assembly and ACP session integration tasks.
