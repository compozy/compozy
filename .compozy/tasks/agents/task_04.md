---
status: pending
title: CLI integration for `exec --agent`, `agents`, and hidden `mcp-serve`
type: backend
complexity: high
dependencies:
  - task_02
  - task_03
---

# Task 04: CLI integration for `exec --agent`, `agents`, and hidden `mcp-serve`

## Overview
Expose reusable agents through Compozy's public CLI by adding agent-aware execution, discovery, and inspection commands, plus the hidden entrypoint that hosts the reserved MCP server over stdio. This task converts the internal agent engine into an operator-facing surface that can be used directly from `compozy exec` and indirectly by ACP runtimes.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists for this feature. Technical requirements are derived from `_techspec.md` and the accepted ADRs.
</critical>

<requirements>
- MUST add `--agent <name>` to `compozy exec` and route the selected agent through the shared resolution and prompt-assembly path.
- MUST add a new `compozy agents` command group with at least `list` and `inspect` subcommands.
- MUST add a hidden `compozy mcp-serve` command that exposes the reserved `compozy` MCP server over stdio for ACP runtime consumption.
- MUST make `agents inspect` report validation results for invalid agents while still printing a usable report before exiting non-zero.
- MUST keep public help output clean by hiding internal-only commands such as `mcp-serve`.
- MUST preserve existing `exec` behavior when `--agent` is omitted.
- MUST ensure explicit CLI flags still win over agent defaults when both are present.
</requirements>

## Subtasks
- [ ] 04.1 Extend the `exec` command state and flag parsing to accept `--agent` and pass the selected agent into the shared execution pipeline.
- [ ] 04.2 Add the `compozy agents list` command to show resolved agents across scopes with workspace/global source attribution.
- [ ] 04.3 Add the `compozy agents inspect` command to print agent details, validation state, and MCP-related diagnostics in a user-actionable format.
- [ ] 04.4 Add the hidden `compozy mcp-serve` command that hosts the reserved MCP server over stdio using the internal engine from task 03.
- [ ] 04.5 Add CLI tests for discovery, selection, inspection, and hidden-command behavior without regressing existing root help and exec usage.

## Implementation Details
See TechSpec "Component Overview" for the CLI surface and "Runtime precedence" for the rule that explicit flags win over agent defaults. The command layer should stay thin: discover and resolve agents through the new registry, then hand off to the existing execution pipeline rather than creating a second code path.

The `agents inspect` output should help a user diagnose invalid frontmatter or MCP config quickly. Preserve the pattern already used elsewhere in the CLI: actionable text output by default, with deterministic exit codes for automation.

### Relevant Files
- `internal/cli/root.go` — Root command tree that will gain `agents` and hidden `mcp-serve`.
- `internal/cli/commands.go` — Current `exec` command definition that needs the `--agent` flag.
- `internal/cli/state.go` — Shared CLI state model that likely needs to carry the selected agent name.
- `internal/cli/root_test.go` — Existing command registration and help output coverage to extend.
- `internal/cli/root_command_execution_test.go` — End-to-end CLI invocation coverage to extend.
- `internal/cli/validate_tasks.go` — Existing reporting conventions for validation-style command output that `agents inspect` should emulate where appropriate.
- `internal/core/kernel/commands/runtime_config.go` — Existing runtime command plumbing that may need the selected agent field threaded through.

### Dependent Files
- `internal/core/run/internal/acpshared/command_io.go` — Shared execution path will consume the selected agent from the CLI layer.
- `internal/core/agents/` — CLI discovery and inspection commands will consume registry results from task 01.
- `README.md` — User-facing command documentation updated later will depend on this final CLI contract.

### Related ADRs
- [ADR-001: Flat Filesystem Agent Definitions with Workspace Override](adrs/adr-001.md) — Governs discovery output for `agents list` and `inspect`.
- [ADR-002: Assemble Agent System Prompts from Resolved Metadata with Explicit Override Precedence](adrs/adr-002.md) — Governs `exec --agent` precedence behavior.
- [ADR-003: Expose Nested Agent Execution Through a Compozy-Owned MCP Server](adrs/adr-003.md) — Governs the hidden `mcp-serve` entrypoint.

## Deliverables
- `exec --agent` CLI support wired into the shared execution path.
- `compozy agents list` and `compozy agents inspect` command surfaces.
- Hidden `compozy mcp-serve` command wired to the internal reserved MCP server.
- CLI tests for public and hidden command behavior.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for CLI command execution **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] `compozy exec --agent council "..."` resolves the selected agent and passes it into the shared execution path.
  - [ ] An unknown agent name passed to `--agent` returns a user-actionable CLI error.
  - [ ] `agents list` shows workspace and global sources distinctly when both are present.
  - [ ] `agents inspect` prints validation details for an invalid `mcp.json` before returning a non-zero exit code.
  - [ ] Root help does not expose the hidden `mcp-serve` command.
- Integration tests:
  - [ ] `compozy exec --agent <name>` preserves existing stdout-mode behavior while using the selected agent's resolved prompt/runtime.
  - [ ] `compozy agents inspect <name>` reports the resolved source, runtime metadata, and validation status for a valid agent.
  - [ ] The hidden `mcp-serve` command can be invoked by tests as a stdio MCP host without requiring public root help exposure.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Users can discover, inspect, and execute reusable agents from the CLI without using internal APIs directly.
- `exec --agent` remains compatible with existing explicit flag behavior and non-agent execution.
- The hidden `mcp-serve` entrypoint exists solely as an internal MCP host and does not clutter public UX.
