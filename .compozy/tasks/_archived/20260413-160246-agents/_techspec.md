# TechSpec: Reusable Agent Definitions and Nested Agent Execution

## Executive Summary

This feature introduces reusable agent definitions to Compozy through flat agent directories discovered from both workspace and global scopes. Each agent directory is anchored by `AGENT.md` using YAML frontmatter plus a markdown body and may include an optional `mcp.json` file using the standard MCP configuration shape. A selected agent becomes a first-class execution target for `compozy exec --agent <name>`, with its own prompt body, runtime defaults, agent-local MCP servers, and a compact discovery catalog of other available agents. The design adapts the agent pattern from agh, but deliberately narrows v1 scope to avoid overfitting: no per-agent `skills/`, no per-agent `memory/`, and no inheritance or composition.

The implementation adds a new agent resolution and prompt-assembly seam to Compozy's existing execution pipeline instead of creating a parallel orchestration stack. Nested agent execution inside ACP-backed sessions is exposed through a single reserved Compozy MCP server named `compozy`, implemented as a hidden stdio subcommand, `compozy mcp-serve`, using the official Go MCP SDK `github.com/modelcontextprotocol/go-sdk/mcp`. That server provides a generic `run_agent` tool and is attached to ACP `session/new` and `session/load` requests together with any MCP servers declared by the selected agent. Child agent invocations create real nested ACP sessions so child `AGENT.md` runtime defaults are honored, while nested-depth and access ceilings stay host-owned rather than tool-controlled. The primary technical trade-off is accepting a small amount of new local orchestration complexity, including nested-run safeguards, agent-local MCP merge rules, and MCP session plumbing, in exchange for a reusable, tool-callable agent model that works across CLI and runtime execution paths. No `_prd.md` exists for this feature, so this TechSpec is based on direct design input plus codebase exploration.

## System Architecture

### Component Overview

- `internal/core/agents` is a new package responsible for parsing `AGENT.md`, loading optional `mcp.json`, discovering global and workspace agents, resolving override precedence, assembling the final system prompt, and serving nested agent execution.
- `internal/cli` extends `compozy exec` with `--agent` and adds a new `compozy agents` command group for discovery and inspection.
- `internal/cli` also adds a hidden internal entrypoint, `compozy mcp-serve`, used only as the stdio command target for the reserved Compozy MCP server.
- `internal/core/agent` remains the ACP transport layer, but its session request types gain merged MCP server configuration so Compozy can attach the reserved `compozy` MCP server and agent-local MCP servers on session creation and session load.
- `internal/core/run/internal/acpshared` becomes the execution seam that combines user prompt, assembled system prompt, runtime overrides, and merged MCP server attachments before opening or resuming ACP sessions.
- `internal/core/workspace` or adjacent discovery helpers resolve `.compozy/agents/<name>/AGENT.md` alongside the existing workspace config path without folding agent definitions into `.compozy/config.toml`.

### Data Flow

1. Compozy starts a command and resolves the workspace root as it does today.
2. Agent discovery loads global agents from `~/.compozy/agents/*/AGENT.md` and workspace agents from `.compozy/agents/*/AGENT.md`.
3. For each discovered agent directory, Compozy loads `AGENT.md` and optional `mcp.json`.
4. The registry merges global and workspace agents by agent name. Workspace override applies to the whole agent directory, not file-by-file.
5. If `--agent <name>` is present, Compozy resolves that agent, merges runtime precedence, and assembles a system prompt from built-in framing, agent metadata, compact discovery, and the agent prompt body.
6. Compozy builds the effective MCP set for the session as `reserved MCP server compozy + agent-local MCP servers from mcp.json`.
7. The reserved `compozy` MCP server is launched as `compozy mcp-serve --server compozy` over stdio and implemented with `github.com/modelcontextprotocol/go-sdk/mcp`.
8. When Compozy opens or loads an ACP session, it attaches that merged MCP set.
9. If an ACP-backed runtime invokes `run_agent`, the MCP server handler resolves the requested child agent, computes the next nested depth from host-owned session state, and starts a real child ACP session with the child agent's own runtime defaults and merged MCP set.
10. The child session receives a fresh reserved `compozy` MCP server instance with the incremented nested depth propagated through host-owned internal metadata and startup environment, not through the tool request.
11. The effective nested access mode is the minimum of the parent session's effective access mode and the child agent's resolved access mode.
12. The child run returns structured output to the parent tool caller.

## Implementation Design

### Core Interfaces

```go
type ResolvedAgent struct {
	Name        string
	Title       string
	Description string
	Source      AgentSource
	Prompt      string
	Runtime     AgentRuntimeDefaults
	MCPConfig   *AgentMCPConfig
}

type Registry interface {
	List(ctx context.Context, workspaceRoot string) ([]ResolvedAgent, error)
	Resolve(ctx context.Context, workspaceRoot, name string) (ResolvedAgent, error)
}
```

```go
type RunAgentRequest struct {
	Name  string `json:"name"`
	Input string `json:"input"`
}

type RunAgentResult struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Output  string `json:"output"`
	RunID   string `json:"run_id,omitempty"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}
```

The important implementation rule is separation of concerns. `internal/core/agents` owns agent definition semantics. `internal/core/agent` continues to own ACP transport semantics. The new code should extend the existing ACP session request surface rather than teach the agent-definition package how to speak ACP directly.

```go
type NestedExecutionContext struct {
	Depth            int
	MaxDepth         int
	ParentRunID      string
	ParentAgentName  string
	ParentAccessMode string
}
```

`NestedExecutionContext` is host-owned state. It is never accepted from MCP tool callers.

### Data Models

**Agent directory structure**

Each reusable agent is represented by a directory at `.compozy/agents/<name>/` or `~/.compozy/agents/<name>/`.

Supported v1 artifacts:

- `AGENT.md` for prompt and runtime defaults
- optional `mcp.json` for agent-local MCP server declarations

Unsupported in v1 and rejected by validation:

- `extends`
- `uses`
- `skills`
- `memory`

**`AGENT.md` metadata**

`AGENT.md` uses YAML frontmatter followed by a markdown body. The frontmatter is the only structured metadata area in v1.

Example:

```md
---
title: Council
description: Multi-advisor decision agent
ide: codex
model: gpt-5.4
reasoning_effort: high
access_mode: read_only
---

You are the council agent. Coordinate multiple expert viewpoints and return a synthesis.
```

Supported v1 metadata fields:

- `title`
- `description`
- `ide`
- `model`
- `reasoning_effort`
- `access_mode`

The markdown body after the metadata block is the base prompt for the agent.

**Agent name validation**

The agent directory name is the canonical agent identifier and must match:

```text
^[a-z][a-z0-9-]{0,63}$
```

Rules:

- lowercase only
- ASCII letters, digits, and hyphen only
- must start with a letter
- maximum length is 64 characters
- names are case-sensitive by construction because only lowercase names are valid
- `compozy` is reserved and cannot be used as an agent name in v1

**`mcp.json` shape**

Compozy uses the standard MCP config shape with a top-level `mcpServers` object, but stores it as `mcp.json` inside the agent directory.

Example:

```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_TOKEN": "${GITHUB_TOKEN}"
      }
    }
  }
}
```

Rules:

- relative executable paths are resolved against the agent directory
- environment placeholders such as `${TOKEN}` are expanded during agent load and session setup
- a missing environment variable is a validation error and fails closed before ACP session creation
- the reserved Compozy MCP server name `compozy` cannot be overridden by `mcp.json`
- when the same agent name exists in both scopes, the winning scope contributes the entire agent directory
- nested child runs do not inherit parent agent-local MCP servers implicitly; each child run uses only the reserved `compozy` MCP server plus the child's own `mcp.json`

**Runtime precedence**

The effective runtime for an agent-backed execution resolves in this order:

1. explicit CLI flags for the current invocation
2. resolved `AGENT.md` runtime defaults
3. workspace `.compozy/config.toml` defaults and command overrides
4. built-in runtime defaults from `model.RuntimeConfig.ApplyDefaults()`

Implementation note:

- explicit CLI precedence must use `cmd.Flags().Changed(name)` so zero values do not accidentally override agent defaults

**Discovery catalog**

The prompt-visible catalog for progressive discovery is intentionally compact and includes only:

- agent name
- short description
- source scope

**System prompt assembly template**

The assembler must emit the final system prompt in this canonical order:

```text
[Compozy built-in mode framing]

<agent_metadata>
name: <agent-name>
title: <agent-title>
description: <agent-description>
source: <workspace|global>
</agent_metadata>

<available_agents>
- <name>: <short-description> (<source>)
...
</available_agents>

[agent markdown body from AGENT.md]
```

The boundaries above are contractual. Implementations may vary whitespace minimally, but not section order or section naming.

**Effective session MCP list**

The effective MCP list for any agent-backed ACP session resolves in this order:

1. reserved Compozy MCP server `compozy`
2. selected agent's `mcp.json` declarations

This is a merge, not an override. Name collisions with the reserved `compozy` MCP server are validation errors.

**Nested execution records**

Nested runs should record:

- parent run id
- parent agent name
- child agent name
- nested depth
- effective runtime and access mode
- completion status
- blocked reason when applicable (`depth_limit`, `cycle_detected`, `capability_rejected`, `access_ceiling`, `runtime_unsupported`)

This data belongs in run artifacts and structured logs, not in a separate agent memory store.

### API Endpoints

There are no HTTP endpoints in this design. The external operational surface is CLI plus MCP.

**CLI surface**

- `compozy exec --agent <name> [prompt]`
  - Executes the selected reusable agent through the existing exec pipeline.
  - Requires prompt input from the positional argument, `--prompt-file`, or stdin, exactly as `exec` does today.
- `compozy agents list`
  - Lists resolved agents with name, description, scope, effective runtime summary, and whether `mcp.json` is present.
- `compozy agents inspect <name>`
  - Shows the resolved source, metadata, runtime defaults, prompt path, and MCP server summary for one agent.
  - Fails with a non-zero exit code on validation errors, while still printing the validation report for the inspected agent.

**MCP tool surface**

- `run_agent`
  - Input fields: `name`, `input`
  - Returns: resolved agent metadata, final output, success flag, run id when available, and failure details on error

The v1 surface deliberately stops here. It does not add one tool per agent and it does not expose agent mutation or installation commands over MCP.

## Integration Points

- ACP session setup is the main external protocol boundary. Compozy must pass merged MCP servers on both `session/new` and `session/load` so fresh and resumed sessions retain the same capability surface.
- Agent-local MCP configuration is read from `mcp.json` using the standard `mcpServers` schema and converted into ACP `mcpServers` entries before session setup.
- The reserved Compozy MCP server is exactly one server in v1, named `compozy`. It is launched as a local stdio subprocess via the hidden command `compozy mcp-serve --server compozy`.
- The reserved Compozy MCP server must be attached to every ACP session Compozy creates, including ordinary skill-driven sessions with no selected reusable agent, so bundled skills can call `run_agent` uniformly across drivers.
- The MCP server implementation uses the official Go SDK, `github.com/modelcontextprotocol/go-sdk/mcp`.
- The reserved Compozy MCP server does not require external authentication because it is local-only and scoped to the current command.
- Filesystem integration is required for two roots:
  - workspace agents under `.compozy/agents/`
  - global agents under `~/.compozy/agents/`
- `compozy setup` provisions the built-in council advisor roster into `~/.compozy/agents/` so skills such as `cy-idea-factory` can rely on a consistent cross-driver council baseline without mirroring driver-native agent directories.
- Each agent directory may contain:
  - `AGENT.md`
  - optional `mcp.json`
- Consumer skills such as `cy-idea-factory` become integration clients of the new capability once the runtime can call `run_agent` through the attached MCP server.

## Impact Analysis

| Component | Impact Type | Description and Risk | Required Action |
|-----------|-------------|---------------------|-----------------|
| `internal/core/agents` | new | Introduces agent parsing, `mcp.json` loading, discovery, resolution, prompt assembly, and nested execution. Medium risk because it becomes a new core orchestration seam. | Add package, tests, and strict validation. |
| `internal/cli/commands.go` and related CLI state | modified | Adds `exec --agent`, `agents` subcommands, and hidden `mcp-serve`. Low to medium risk because flag precedence must stay deterministic and the hidden entrypoint must stay internal-only. | Extend flags, command wiring, and help text. |
| `internal/core/agent/client.go` | modified | Session creation and load must carry merged MCP server definitions from the reserved `compozy` MCP server and agent-local `mcp.json`. Medium risk because ACP bootstrapping is shared by all runtimes. | Extend request types and tests for create/load parity. |
| `internal/core/run/internal/acpshared/command_io.go` | modified | Must compose assembled prompts and merged MCP attachments before session start or load. Medium risk because it sits on the hot execution path. | Add agent-aware session setup and resume coverage. |
| MCP server dependency surface | new | Introduces an explicit Go dependency for serving MCP over stdio and a hidden runtime entrypoint. Medium risk because it affects lifecycle and binary internals. | Add `github.com/modelcontextprotocol/go-sdk/mcp` and wire the hidden server command. |
| `internal/core/workspace` discovery helpers | modified | Workspace-scoped agent lookup joins existing config resolution. Low risk. | Add discovery helpers without folding agents into config TOML. |
| Bundled skills that reference conceptual helper agents | modified | Prompts such as `cy-idea-factory` can be upgraded to call real agents. Low risk after core capability exists. | Update selected skills after the runtime path is in place. |

## Testing Approach

### Unit Tests

- Parse valid and invalid `AGENT.md` files, including unsupported fields.
- Parse valid and invalid `mcp.json` files, including reserved-name collisions and bad shape validation.
- Resolve workspace and global agents with correct override behavior.
- Verify runtime precedence: CLI wins over agent defaults, agent defaults win over workspace config.
- Verify system prompt assembly order and compact discovery catalog output.
- Verify `run_agent` request parsing, result serialization, depth limits, and cycle detection.
- Verify CLI output for `agents list` and `agents inspect`.
- Verify `agents inspect` validation reporting on invalid `AGENT.md` and invalid `mcp.json`.

### Integration Tests

- `compozy exec --agent <name>` should run through the existing exec pipeline with the resolved runtime, assembled system prompt, and merged MCP set.
- ACP session creation should attach merged MCP servers on new sessions.
- ACP session load should also attach merged MCP servers for resumed sessions.
- A fake ACP runtime invoking `run_agent` should trigger a bounded child run and receive structured output.
- Same-name global and workspace agents should resolve to the workspace agent end to end, including `mcp.json`.
- Nested execution should reject cycles end to end, including `A -> B -> A`.
- Nested execution should fail closed with a clear capability error if MCP attachment or runtime support is unavailable.

## Development Sequencing

### Build Order

1. Add `internal/core/agents` data models, parser, and validation for flat `AGENT.md` definitions plus optional `mcp.json`. No dependencies.
2. Add global and workspace discovery plus whole-directory override resolution. Depends on step 1.
3. Add agent prompt assembly and runtime precedence merge for exec-path use. Depends on steps 1 and 2.
4. Add CLI surface for `compozy exec --agent`, `compozy agents list`, and `compozy agents inspect`. Depends on steps 2 and 3.
5. Extend ACP session create and load plumbing to pass merged MCP servers and implement the `run_agent` MCP tool. Depends on steps 2 and 3.
6. Add nested-run safeguards, structured observability, and representative consumer updates for bundled skills. Depends on steps 3, 4, and 5.

### Technical Dependencies

- ACP session requests already support `mcpServers` on both new and load operations, so no protocol change is required.
- The hidden stdio MCP server uses `github.com/modelcontextprotocol/go-sdk/mcp`.
- No database or remote service dependency is required for v1.
- The feature depends on the existing exec pipeline remaining the single source of truth for agent execution rather than introducing a side path.

## Monitoring and Observability

- Emit structured events for `agent_registry_loaded`, `agent_resolved`, `agent_prompt_assembled`, `agent_mcp_attached`, `run_agent_invoked`, `run_agent_completed`, and `run_agent_blocked`.
- Include fields such as `agent_name`, `agent_source`, `run_id`, `parent_run_id`, `nested_depth`, `runtime`, `access_mode`, `session_id`, and `blocked_reason` when available.
- Record duplicate-name override decisions in debug logs so workspace-over-global behavior is visible.
- Record nested-run failures in the same run artifacts used by exec and workflow commands.
- There is no external alerting system in v1. Operational thresholds are local warnings and exit failures:
  - warning on unsupported fields in discovered agent roots
  - warning on duplicate names across scopes
  - hard failure on recursion depth limit or unsupported MCP attachment
  - hard failure on reserved host MCP name collisions in `mcp.json`

## Technical Considerations

### Key Decisions

- Agent definitions stay file-based and flat, but the agent directory may include optional `mcp.json`.
- `mcp.json` uses the standard MCP `mcpServers` object shape rather than a Compozy-specific schema.
- `AGENT.md` uses YAML frontmatter plus markdown body, matching the repository's existing frontmatter conventions.
- Agent prompt assembly is additive and mode-aware instead of treating `AGENT.md` as an opaque replacement system prompt.
- Workspace override applies to the full agent directory, not to individual files inside the directory.
- Nested agent execution is delivered through one generic MCP tool, `run_agent`, while preserving agent-local MCP declarations for the selected child agent.
- The reserved Compozy MCP server is exactly one server in v1, named `compozy`, launched as the hidden stdio subcommand `compozy mcp-serve`.
- Child agent execution creates a real nested ACP session rather than an inline shortcut so child runtime defaults remain authoritative.
- Nested-depth is host-owned state propagated through internal session metadata and MCP server startup environment, not caller-controlled tool input.
- Nested access can only stay equal or become more restrictive; it never escalates above the parent session's effective access mode.
- MCP server attachment applies to both ACP new and load paths so persisted exec sessions do not lose the capability surface on resume.
- The first rollout targets all ACP-backed sessions created by Compozy and fails closed when a runtime rejects MCP registration.

### Known Risks

- Runtime-specific MCP behavior may differ even when the ACP protocol surface exists.
  - Mitigation: cover supported runtimes with integration tests and degrade with explicit capability errors.
- Recursive nested runs can create runaway execution trees.
  - Mitigation: enforce maximum depth, detect cycles, and carry parent metadata through nested calls.
- Hidden MCP server lifecycle bugs could strand subprocesses if startup or shutdown is incomplete.
  - Mitigation: use a single stdio subprocess model with explicit start/stop ownership in the ACP session lifecycle and integration coverage for teardown.
- Prompt size may grow when many agents exist.
  - Mitigation: keep progressive discovery compact and avoid dumping full prompt bodies or runtime details for sibling agents.
- Reserved host MCP names may collide with agent-local declarations.
  - Mitigation: validate agent-local `mcp.json` and fail on collision before runtime.
- Users may expect parent agent-local MCP servers to flow into child runs automatically.
  - Mitigation: document that each child run rebuilds its MCP set from the child agent directory only.
- Missing environment variables in `mcp.json` could create confusing partial runtime failures.
  - Mitigation: expand variables during agent load and fail closed before ACP session creation when a required variable is absent.
- Users may expect agh parity immediately.
  - Mitigation: reject unsupported v1 fields clearly and document the deferred features in CLI help and the saved TechSpec.

## Architecture Decision Records

- [ADR-001: Flat Filesystem Agent Definitions with Workspace Override](adrs/adr-001.md) — Reusable agents live in flat directories under workspace and global scopes, with whole-directory workspace override.
- [ADR-002: Assemble Agent System Prompts from Resolved Metadata with Explicit Override Precedence](adrs/adr-002.md) — Agent execution uses an additive system prompt assembler and a deterministic runtime precedence chain.
- [ADR-003: Expose Nested Agent Execution Through a Compozy-Owned MCP Server](adrs/adr-003.md) — ACP-backed runtimes invoke nested agents through a generic `run_agent` MCP tool attached during session setup.
- [ADR-004: Keep V1 Agent Capability Scope Minimal](adrs/adr-004.md) — V1 excludes per-agent skills, memory, inheritance, and composition.
- [ADR-005: Use Agent-Local `mcp.json` with Standard MCP Configuration Shape](adrs/adr-005.md) — Agent-local MCP configuration lives in `mcp.json`, using the standard `mcpServers` schema.
