Goal (incl. success criteria):
- Produce a TechSpec for reusable `.compozy/agents` in Compozy, adapted from the AGH agent pattern, covering CLI execution via `compozy exec --agent <name>` and ACP-driver execution via tools, with enough architectural detail to guide implementation and later task breakdown.

Constraints/Assumptions:
- Follow `AGENTS.md`, `CLAUDE.md`, and the `cy-create-techspec` skill workflow.
- This is design/spec work only for now; do not implement code in this session unless the user explicitly pivots.
- Required skills in use: `cy-create-techspec`, `brainstorming`, `find-docs`.
- Runtime lacks the dedicated blocking question tool in this mode, so required clarification questions must be asked as plain assistant messages and execution must stop until the user responds.
- Existing ledgers in `.codex/ledger/` are unrelated completed work; keep read-only awareness and do not modify them.

Key decisions:
- Use the AGH RFC and implementation as reference input, but adapt the design to Compozy's current package layout and execution pipeline rather than copying package boundaries mechanically.
- Keep the scope centered on agent definition, resolution, and execution contracts; avoid speculative expansion into unrelated workflow features unless the current architecture requires it.
- User clarified the desired v1 scope:
  - no per-agent `skills/` support yet
  - no per-agent `memory/` support yet
  - yes to a system-prompt assembly layer that injects agent metadata and supports progressive discovery of reusable agents
  - yes to CLI commands for inspecting/managing agents
  - yes to execution integration (`compozy exec --agent <name>` and ACP-driver/tool integration)
- User confirmed that `AGENT.md` should define the base runtime for `compozy exec --agent <name>` (for example `ide`, `model`, `reasoning_effort`, `access_mode`), while explicit CLI flags override those defaults.
- User confirmed agent discovery should exist in both scopes:
  - workspace: `.compozy/agents/<name>/AGENT.md`
  - global: `~/.compozy/agents/<name>/AGENT.md`
- User confirmed workspace scope should override global scope when the same agent name exists in both places.
- User confirmed ACP-driver integration should use a generic tool surface rather than generating one dedicated tool per agent.
- User confirmed agent-local MCP configuration should exist in v1 as `mcp.json` inside the agent directory, following the standard MCP config shape rather than inline declarations in `AGENT.md`.
- ACP protocol research confirmed that ACP sessions already support MCP server configuration via `session/new`/`mcpServers`; therefore the generic agent tool can plausibly be delivered as an MCP tool exposed by a Compozy-provided MCP server, instead of requiring bespoke ACP host-command registration.
- The remaining implementation work is plumbing, not protocol invention: Compozy must create/configure that MCP server, pass it into ACP session setup, and handle recursion/policy/result marshaling.
- Draft direction now assumes the Compozy MCP server is attached on both ACP session creation and session load/resume paths, because both `NewSessionRequest` and `LoadSessionRequest` support `mcpServers` and current Compozy code leaves both empty.
- Rollout direction chosen for the draft: inject the Compozy MCP server for all ACP-backed sessions created by Compozy, and fail closed with a clear capability error if a runtime rejects MCP server registration.
- Follow-up review decisions now incorporated into the artifact set:
  - `AGENT.md` is explicitly `YAML frontmatter + markdown body`
  - agent names use slug validation `^[a-z][a-z0-9-]{0,63}$`
  - the reserved MCP server is exactly one server in v1, named `compozy`
  - the reserved MCP server is implemented as hidden stdio subcommand `compozy mcp-serve`
  - the MCP server implementation uses the official Go SDK `github.com/modelcontextprotocol/go-sdk/mcp`
  - `run_agent` creates real nested ACP sessions; nested depth is host-owned and not part of the tool contract
  - nested access mode cannot escalate above the parent effective access mode
  - `mcp.json` environment placeholders expand at load/setup time and fail closed when variables are missing
  - CLI precedence explicitly relies on `cmd.Flags().Changed(...)`
- Clarified via ACP docs that passing `mcpServers` in `session/new` or `session/load` only tells the agent where to connect; it does not synthesize host-owned tools. Agent-local `mcp.json` therefore covers external MCP servers, while the reserved `compozy` MCP server exists only to expose Compozy-owned tools like `run_agent`.
- User selected the full v1 direction rather than the reduced variant: keep `exec --agent`, prompt assembly, discovery, agent-local `mcp.json`, and the generic `run_agent` tool exposed through the reserved Compozy MCP server.

State:
- Completed. The `agents` task breakdown was approved, the task artifacts were generated, `go run ./cmd/compozy validate-tasks --name agents` passed, and `make verify` passed.

Done:
- Read the explicit user request and the provided `cy-create-techspec` skill instructions.
- Read the `brainstorming` skill because the request introduces a new feature.
- Read the `find-docs` skill because the user requested protocol research against current documentation.
- Scanned existing session ledgers under `.codex/ledger/` for cross-agent awareness; all visible entries are from unrelated completed tasks.
- Created an execution plan aligned to the TechSpec workflow.
- Mapped the current Compozy execution seam:
  - `internal/cli/commands.go` exposes `compozy exec [prompt]` but has no reusable agent selector yet.
  - `internal/cli/state.go` and `internal/core/model/runtime_config.go` treat `IDE` as the ACP runtime choice (`codex`, `claude`, etc.), not as a reusable agent definition.
  - `internal/core/agent/*` is a registry of ACP transport launch specs, availability checks, and tool-call normalization, not a higher-level agent concept.
  - `internal/core/plan/prepare.go` resolves work items and builds prompts; prompt composition today is mode-specific in `internal/core/prompt/*`.
  - `internal/core/memory/store.go` provides workflow/task memory for PRD tasks only; exec mode currently has no agent-scoped memory layer.
  - `internal/core/workspace/*` resolves `.compozy/config.toml` defaults but not workspace-defined agents.
- Mapped the AGH reference seam:
  - `internal/config/agent.go` loads `AGENT.md` definitions with prompt + runtime metadata.
  - `internal/workspace/scanner.go` and `resolver.go` discover agents per workspace and merge them with other workspace resources.
  - `internal/session/manager_start.go` resolves an agent definition before session start and injects the assembled prompt into ACP start options.
  - `internal/daemon/composed_assembler.go` and `internal/memory/assembler.go` show how AGH layers prompt providers and memory around a base agent prompt.
- Confirmed there is no existing `_prd.md` for this feature in `.compozy/tasks/`; current task directories are `auto-update`, `extensibility`, and `refac`.
- Narrowed the target shape based on user feedback: this is no longer the AGH-style full agent package; it is an agent-definition and discovery/execution feature with prompt assembly and CLI support, but without agent-local skills or memory in v1.
- Verified ACP support in authoritative docs:
  - official ACP docs and Context7 show `session/new` accepts `mcpServers`
  - current Compozy code still initializes ACP sessions with `McpServers: []acp.McpServer{}` in `internal/core/agent/client.go`
  - current ACP shared session creation path in `internal/core/run/internal/acpshared/command_io.go` does not yet thread reusable MCP server configuration through execution setup
- Assumed the task slug `.compozy/tasks/agents/` for this design artifact set.
- Created the ADR directory and initial accepted ADRs:
  - `adr-001.md` — flat filesystem agent definitions with workspace override
  - `adr-002.md` — system prompt assembly and runtime precedence
  - `adr-003.md` — ACP nested execution through a Compozy-owned MCP server
  - `adr-004.md` — explicit minimal v1 capability scope
  - `adr-005.md` — agent-local `mcp.json` with standard MCP configuration shape
- Reviewed the follow-up design feedback against the saved TechSpec and ADRs.
- Confirmed the strongest adjustment points are valid before task creation:
  - MCP server lifecycle/SDK choice is still ambiguous
  - nested `run_agent` execution mode and depth propagation are underspecified
  - `RunAgentRequest.MaxDepth` should not stay caller-controlled
  - `AGENT.md` format must be declared explicitly
  - system-prompt assembly needs a concrete output template
  - nested access-mode downgrading must be specified
  - environment-variable expansion behavior in `mcp.json` must be specified
  - agent name validation must be specified
  - CLI precedence should explicitly call out `cmd.Flags().Changed(...)`
- Confirmed with local code that:
  - the repository already has YAML frontmatter support in `internal/core/frontmatter`
  - the CLI already relies on `cmd.Flags().Changed(...)` to distinguish explicit flags from zero values
- Revised `.compozy/tasks/agents/_techspec.md` to resolve the accepted review points.
- Revised ADRs `adr-001.md`, `adr-002.md`, `adr-003.md`, and `adr-005.md` to align with the tightened design.
- Clarified the architectural distinction between agent-local MCP configuration and the reserved Compozy MCP server:
  - `mcp.json` is sufficient for external MCP servers declared by an agent
  - the reserved Compozy MCP server is only needed to expose host-owned tools such as `run_agent`
- Confirmed with the user that v1 should keep the full scope, including the generic nested `run_agent` tool exposed through the reserved MCP server.
- Read the `cy-create-tasks` skill, task template, and task context schema to align the breakdown and generated files with repository conventions.
- Confirmed there is no workspace `.compozy/config.toml`, so task types use the built-in registry.
- Generated the approved task artifacts:
  - `.compozy/tasks/agents/_tasks.md`
  - `.compozy/tasks/agents/task_01.md`
  - `.compozy/tasks/agents/task_02.md`
  - `.compozy/tasks/agents/task_03.md`
  - `.compozy/tasks/agents/task_04.md`
  - `.compozy/tasks/agents/task_05.md`
  - `.compozy/tasks/agents/task_06.md`
- Validated the generated task files successfully with `go run ./cmd/compozy validate-tasks --name agents`.
- Re-ran repository verification successfully with `make verify`.

Now:
- None.

Next:
- None.

Open questions (UNCONFIRMED if needed):
- None currently blocking.

Working set (files/ids/commands):
- `.codex/ledger/2026-04-10-MEMORY-compozy-agents-techspec.md`
- `.compozy/tasks/agents/adrs/{adr-001.md,adr-002.md,adr-003.md,adr-004.md}`
- `/Users/pedronauck/Dev/compozy/looper/.agents/skills/cy-create-techspec/SKILL.md`
- `/Users/pedronauck/Dev/compozy/looper/.agents/skills/brainstorming/SKILL.md`
- `/Users/pedronauck/.agents/skills/find-docs/SKILL.md`
- `.codex/ledger/*-MEMORY-*.md`
- `internal/cli/{commands.go,run.go,state.go,workspace_config.go}`
- `internal/core/{api.go,workflow_target.go}`
- `internal/core/agent/{client.go,registry_specs.go,registry_validate.go,registry_launch.go,acp_convert.go}`
- `internal/core/plan/prepare.go`
- `internal/core/prompt/{common.go,prd.go,review.go}`
- `internal/core/memory/store.go`
- `internal/core/workspace/{config.go,config_types.go}`
- `internal/core/run/internal/acpshared/command_io.go`
- `/Users/pedronauck/Dev/projects/agh/docs/rfcs/001_agent-md-with-skills-memory.md`
- `/Users/pedronauck/Dev/projects/agh/internal/{config/agent.go,workspace/{resolver.go,scanner.go},session/{manager_start.go,manager_helpers.go,manager_workspace.go},daemon/composed_assembler.go,memory/assembler.go,cli/agent.go}`
- External docs: `https://agentclientprotocol.com/protocol/session-setup`, `https://agentclientprotocol.com/protocol/schema`, `https://agentclientprotocol.com/get-started/architecture`, `https://agentclientprotocol.com/rfds/mcp-over-acp`
- Commands: `npx -y ctx7@latest library \"Agent Client Protocol\" \"MCP servers session setup\"`, `npx -y ctx7@latest docs /agentclientprotocol/agent-client-protocol \"MCP servers session setup McpServer CreateSession NewSessionRequest\"`
