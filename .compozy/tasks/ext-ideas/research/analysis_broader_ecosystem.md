# Broader AI Coding Agent Ecosystem -- Extension Analysis

> Research date: 2026-04-11
> Focus: Extension/plugin mechanisms in AI coding tools other than Claude Code

---

## Cursor

### Extension Mechanisms

Cursor provides five distinct extensibility layers:

1. **Project Rules (`.cursor/rules/*.mdc`)** -- Markdown files with YAML frontmatter stored in `.cursor/rules/` directory. These are version-controlled, per-project instructions that guide AI behavior. Each rule file can specify glob patterns for file targeting, making them context-aware (e.g., a rule for `*.go` files vs `*.tsx` files).

2. **User Rules** -- Global rules configured via Cursor settings that apply across all projects. These define personal coding preferences and style guidelines.

3. **Legacy `.cursorrules`** -- A single-file approach (now deprecated in favor of the `.cursor/rules/` directory) that was the original community-driven customization mechanism. Still widely used.

4. **Model Context Protocol (MCP)** -- Full MCP client support allowing Cursor to connect to external MCP servers. Configuration is done via `.cursor/mcp.json` or global settings. Supports both `stdio` and `sse` transport types.

5. **VS Code Extension Compatibility** -- As a VS Code fork, Cursor supports the entire VS Code extension ecosystem, giving it access to thousands of existing extensions.

6. **Hooks** -- Cursor supports hooks (configured via `hooks/hooks.json`) that can trigger actions at specific points in the development workflow.

7. **Skills** -- A newer mechanism (found via `skills/*/SKILL.md` files) that packages reusable AI capabilities.

8. **Agents** -- Custom agent definitions stored as markdown files in an `agents/` directory.

9. **LSP Servers** -- Custom language server protocol configurations via `.lsp.json` files.

### Notable Extensions

- **cursor.directory** (https://github.com/pontusab/cursor-directory) -- The primary community hub for sharing Cursor plugins, rules, MCP servers, skills, agents, and hooks. Uses the "Open Plugins" standard for automatic detection of components from GitHub repos. Backed by Supabase; 13k+ stars.
- **awesome-cursorrules** (https://github.com/PatrickJS/awesome-cursorrules) -- Curated collection of `.cursorrules` files organized by framework/language:
  - Frontend: Next.js, React, Vue 3, Nuxt, SvelteKit, Angular
  - Backend: FastAPI, Django, Flask, Go (Fiber, ServeMux), Laravel, Rails
  - Testing: Cypress, Playwright, Jest, Vitest
  - Full-stack: TypeScript + various frameworks
- **MCP Servers** -- Cursor users widely adopt MCP servers from the awesome-mcp-servers ecosystem (84.5k stars). Popular ones include database connectors, API bridges, browser automation, and Jira/GitHub integrations.

### Patterns Worth Adopting

- **Per-file-pattern rules**: Rules that activate only when editing files matching specific globs (e.g., `*.go`, `*.test.ts`). This is more granular than a single monolithic instruction file.
- **Open Plugins standard**: Automatic discovery of extension components from git repos via standardized file patterns. No manual registration -- just follow the convention.
- **Skills as packaged capabilities**: Bundled markdown+config files that teach the AI specific domain knowledge.
- **Agent definitions as markdown**: Declarative agent personas in version-controlled markdown files.
- **Community marketplace model**: cursor.directory as a web-based hub for discovering and sharing rules, not requiring PR-based contribution workflows.

---

## Windsurf (Codeium)

### Extension Mechanisms

1. **Cascade** -- Windsurf's primary AI agent, supporting two modes:
   - **Code mode**: Creates and modifies code in the codebase
   - **Chat mode**: Answers questions and proposes code without modifying files

2. **Memories** -- Persistent context that Windsurf learns over time. The agent remembers project-specific knowledge across sessions, eliminating the need to re-explain project context.

3. **Rules** -- Customizable instructions that guide Cascade behavior:
   - **Global rules**: Apply across all workspaces
   - **Workspace rules**: Project-specific behavior guidelines

4. **MCP Servers** -- Windsurf supports MCP servers to extend agent capabilities with external tools and services. Configuration follows the standard MCP protocol.

5. **Workflows** -- Automation system for "repetitive trajectories." Users define reusable workflows to streamline common development patterns (build, test, deploy sequences).

6. **`.codeiumignore` files** -- Repository-level or global files that prevent the agent from accessing specified paths (similar to `.gitignore` for AI context).

7. **VS Code Extension Compatibility** -- As a VS Code fork, supports importing settings and extensions from VS Code or Cursor.

8. **Checkpoints** -- Named snapshots of project state that enable safe experimentation with easy reversion.

9. **Tool Integration** -- Supports up to 20 tool calls per prompt including search, web search, MCP servers, terminal access, and linter integration. Auto-fix linting is enabled by default.

### Notable Extensions

- Windsurf's extension ecosystem is younger than Cursor's, with fewer community-specific resources
- The primary differentiator is the built-in **Workflows** system for automation
- **Auto-Continue** feature automatically resumes agent work when tool-call limits are hit
- **Queued messages** allow stacking requests while Cascade works

### Patterns Worth Adopting

- **Memories as persistent learned context**: Rather than just static rules, the agent learns and remembers project-specific patterns over time. This is a step beyond file-based rules.
- **Workflows for repeatable trajectories**: Codifying common multi-step processes (like "lint, test, build, deploy") as reusable automation templates.
- **`.codeiumignore` for AI context boundaries**: A simple, gitignore-style mechanism to control what the AI can and cannot see. Practical for sensitive code or irrelevant generated files.
- **Auto-fix linting integration**: Automatically running linters after AI edits and fixing issues without user intervention.
- **Checkpoint-based experimentation**: Named snapshots that let users safely explore different approaches and revert.

---

## Aider

### Extension Mechanisms

1. **Configuration System** -- Multi-layered configuration via:
   - Command-line switches (`aider --dark-mode`)
   - YAML config file (`.aider.conf.yml`) in home directory or repo root
   - Environment variables (`AIDER_xxx`)
   - `.env` files for local environment configuration
   - Files loaded in priority order with later entries overriding earlier ones

2. **Conventions Files** -- Markdown files (e.g., `CONVENTIONS.md`) that specify coding guidelines:
   - Library preferences (e.g., httpx over requests)
   - Code style requirements (e.g., type hints)
   - Testing conventions and documentation standards
   - Loaded via `--read CONVENTIONS.md` flag or `read:` config option
   - Community repository at https://github.com/Aider-AI/conventions (189 stars)

3. **Watch Mode (`--watch-files`)** -- File monitoring for AI coding comments:
   - `# AI!` comments trigger code modifications
   - `# AI?` comments trigger answers/explanations
   - Works across Python (`#`), JavaScript (`//`), SQL (`--`) comment styles
   - Multiple `# AI` comments coordinate across files before a final `AI!` trigger
   - Enables IDE-agnostic integration -- works in any editor

4. **Lint & Test Integration**:
   - Built-in linters for most popular languages
   - `--lint-cmd` for custom linter commands per language
   - `--test-cmd` for custom test commands
   - `--auto-lint` and `--auto-test` for automatic post-edit validation
   - Automatic fix attempts when linting or testing fails

5. **Python Scripting API** (unofficial):
   ```python
   from aider.coders import Coder
   from aider.models import Model
   coder = Coder.create(main_model=model, fnames=fnames)
   coder.run("instruction text")
   ```
   - Sequential instruction execution
   - Supports in-chat commands
   - Caveat: API is not officially supported and may change without notice

6. **Git Integration**:
   - Automatic commits with descriptive messages
   - Custom commit message prompts via `--commit-prompt`
   - Co-authored-by trailers via `--attribute-co-authored-by`
   - Dirty file handling with separate commits for pre-existing changes

7. **Advanced Model Settings** (`.aider.model.settings.yml`):
   - Custom edit formats (whole, diff, udiff)
   - Weak/editor model configuration
   - Repository map control
   - Cache control and streaming settings
   - `extra_params` for arbitrary litellm.completion() options

8. **Multi-LLM Support**: Works with Claude, GPT-4o, DeepSeek, Gemini, local models via Ollama, and any OpenAI-compatible API.

9. **Browser Interface** (`aider --browser`): Experimental web UI for browser-based interaction.

### Notable Extensions

- **Aider Conventions Repository** (https://github.com/Aider-AI/conventions):
  - `bash-scripts/` -- Shell scripting standards
  - `flutter/` -- Flutter mobile development conventions
  - `functional-programming/` -- FP paradigm conventions
  - `golang/` -- Go language conventions
  - `nextjs-ts/` -- Next.js + TypeScript conventions
  - `moodle500/` -- Moodle 5.0 platform conventions
  - `icalendar-events/` -- Calendar event handling

- **Community Scale**: 43.1k GitHub stars, 5.7M+ PyPI installs, 15B tokens/week, top 20 on OpenRouter

### Patterns Worth Adopting

- **Watch mode with in-file AI comments**: The `AI!` / `AI?` comment pattern is elegant -- developers mark exactly where they want changes in their IDE, and Aider picks them up. IDE-agnostic by design.
- **Conventions files as read-only context**: Loading coding guidelines as read-only files (not editable by the AI) that influence behavior without cluttering the conversation.
- **Community conventions repository**: A dedicated repo for sharing coding conventions with a clear contribution process (subdirectory + README + CONVENTIONS.md).
- **Lint-then-fix loop**: Automatic linting after edits with AI-driven fix attempts. Simple but effective quality gate.
- **Git-aware dirty file handling**: Committing pre-existing changes separately from AI changes to maintain clean git history.
- **Scripting API for automation**: Even unofficial, the ability to programmatically drive the coding agent enables CI/CD integration and batch processing.

---

## Continue.dev

### Extension Mechanisms

1. **Model Roles** -- Six specialized roles with independently configurable models:
   - **Chat**: Conversational AI interactions
   - **Autocomplete**: Code completion suggestions
   - **Edit**: Code modification and refactoring
   - **Apply**: Implementing suggested changes
   - **Embeddings**: Semantic search and code understanding
   - **Rerank**: Relevance refinement of retrieved context

2. **Context Providers** -- Pluggable data sources that feed context to the AI. Built-in providers include:
   - **File/Code**: `ClipboardContextProvider`, `CurrentFileContextProvider`, `FileTreeContextProvider`, `FolderContextProvider`, `OpenFilesContextProvider`, `CodebaseContextProvider`
   - **Version Control**: `GitCommitContextProvider`, `GitHubIssuesContextProvider`, `GitLabMergeRequestContextProvider`
   - **Issue Tracking**: `JiraIssuesContextProvider`
   - **Development**: `TerminalContextProvider`, `ProblemsContextProvider`, `DebugLocalsProvider`, `DiffContextProvider`
   - **External Services**: `DatabaseContextProvider`, `PostgresContextProvider`, `GoogleContextProvider`, `HttpContextProvider`
   - **System**: `OSContextProvider`, `RulesContextProvider`, `CustomContextProvider`
   - **MCP**: `MCPContextProvider` for Model Context Protocol integration
   - **Web**: `WebContextProvider`, `URLContextProvider`
   - **Search**: `SearchContextProvider`, `RepoMapContextProvider`, `DocsContextProvider`

3. **Rules System** -- Multi-layered instruction system:
   - **Local rules**: `.continue/rules/` folder in workspace (markdown with YAML frontmatter)
   - **Hub rules**: Managed on Continue Mission Control, referenced via `config.yaml`
   - **Global rules**: `~/.continue/rules/` for user-wide settings
   - Rule properties: `name`, `globs` (file patterns), `regex` (content patterns), `description`, `alwaysApply`
   - Loading order: Hub assistant rules -> Hub referenced rules -> Local workspace rules -> Global rules

4. **MCP Server Integration**:
   - Configure via `.continue/mcpServers/` folder with YAML files
   - Supports `stdio`, `sse`, and `streamable-http` transport types
   - **JSON compatibility**: Can directly use MCP config files from Claude Desktop, Cursor, or Cline
   - Secrets management via template syntax: `${{ secrets.API_KEY }}`
   - MCP tools available in **agent mode only**

5. **Tool System** -- Two categories:
   - **Base tools** (always available): File read/create, terminal commands, search, diff, directory listing, rule creation, URL fetching
   - **Config-dependent tools**: Conditionally loaded based on auth status, model capabilities, feature flags, and environment type
   - Model-aware tool selection (e.g., `multiEditTool` only if model supports it)
   - Authentication-gated features (web search requires sign-in)

6. **Hub (Mission Control)** -- Central platform for managing configurations, sharing assistants, and distributing rules across teams.

7. **Multi-IDE Support** -- Extensions for VS Code, JetBrains, and CLI (`cn` tool).

8. **Source-Controlled AI Checks** -- `.continue/checks/` directory with markdown files that define AI checks executable as agents on pull requests.

### Notable Extensions

- **Continue Hub**: Centralized configuration management and sharing platform
- **Built-in context providers**: 25+ providers covering git, databases, Jira, GitLab, Google, and more
- **CLI tool (`cn`)**: Source-controlled AI checks enforceable in CI pipelines
- **GitHub**: https://github.com/continuedev/continue -- TypeScript-based (84.4%), with extensions/ directory for VS Code, JetBrains, and CLI

### Patterns Worth Adopting

- **Context providers as a pluggable architecture**: Each data source is a separate provider with a standard interface. This is highly composable -- users pick which providers they need.
- **Model roles for task specialization**: Assigning different models to different tasks (chat, autocomplete, edit, apply) based on performance and cost. This maps well to Compozy's multi-agent architecture.
- **Rule loading order with precedence**: Hub -> workspace -> global, giving team-level rules priority while allowing individual customization.
- **Cross-tool MCP config compatibility**: Accepting MCP config files from Cursor, Claude Desktop, and Cline without modification reduces adoption friction.
- **CI-enforceable AI checks**: Using AI agents as automated code reviewers in CI pipelines via source-controlled check definitions.
- **Secrets templating in MCP configs**: `${{ secrets.API_KEY }}` syntax for safe credential injection into MCP server configurations.

---

## Cline

### Extension Mechanisms

1. **Model Context Protocol (MCP)** -- First-class MCP support with unique "conversational tool creation":
   - Users can say "add a tool that fetches Jira tickets" and Cline will create, install, and configure a new MCP server automatically
   - Community-made MCP servers available through official registry
   - MCP service architecture with dedicated modules: `McpHub.ts` (orchestrator), `McpOAuthManager.ts` (auth), `StreamableHttpReconnectHandler.ts` (reliability)
   - OAuth support for secure MCP server authentication

2. **Custom Instructions (`.clinerules`)** -- Project-level instruction files that guide Cline's behavior, similar to `.cursorrules`.

3. **Built-in Tools**:
   - **File operations**: Create, edit with diff preview, linter/compiler error monitoring
   - **Terminal integration**: VSCode shell integration API for command execution with real-time output
   - **Browser automation**: Claude Computer Use for launching browsers, clicking elements, capturing screenshots and console logs
   - **Context references**: `@url`, `@problems`, `@file`, `@folder` for efficient context management

4. **Checkpoints** -- Workspace state snapshots for comparing diffs and restoring to previous points.

5. **Multi-Provider API Support**: OpenRouter, Anthropic, OpenAI, Google Gemini, AWS Bedrock, Azure, GCP Vertex, Cerebras, Groq, OpenAI-compatible APIs, local models via LM Studio/Ollama.

6. **Subagent Configuration** -- `AgentConfigLoader` for file-based agent setup with configurable modelId and skills.

7. **Loop Detection** -- Prevents infinite tool call sequences, an important safety mechanism.

8. **Kanban Interface** -- Task management view for organizing development work.

### Notable Extensions

- **Cline MCP Marketplace** -- Community-contributed MCP servers available directly within the extension
- **Auto-generated MCP servers** -- Cline can scaffold complete MCP servers from natural language descriptions
- **Docker MCP Toolkit** -- Community integration for containerized development
- **Figma MCP** -- Design-to-code integration via MCP
- **GitHub**: https://github.com/cline/cline -- 60.1k stars, 6.1k forks, Apache 2.0 license

### Patterns Worth Adopting

- **Conversational tool creation**: "Add a tool that..." pattern is remarkably user-friendly. The AI agent itself creates the extension, dramatically lowering the barrier to extensibility.
- **Built-in MCP marketplace**: Discovering and installing MCP servers from within the tool itself rather than requiring manual configuration.
- **OAuth-aware MCP integration**: Supporting OAuth flows for MCP servers that require authentication (enterprise APIs, cloud services).
- **Loop detection / infinite recursion prevention**: Safety mechanism to prevent runaway tool call chains. Essential for any agentic system.
- **Browser automation as a built-in tool**: Using Computer Use to interact with web apps directly, enabling visual debugging and end-to-end testing.
- **Subagent configuration files**: File-based setup for specialized sub-agents with different models and capabilities.

---

## Codex CLI (OpenAI)

### Extension Mechanisms

1. **AGENTS.md** -- Instruction files (similar to CLAUDE.md) that define development standards:
   - Placed in repository root
   - Contains coding style, naming conventions, architecture rules, testing patterns
   - Loaded automatically when Codex runs in a repository
   - Example content: Rust naming conventions, module size targets (<500 LOC), API design patterns, snapshot testing requirements

2. **Configuration via `config.toml`**:
   - Located at `~/.codex/config.toml`
   - MCP server connections with per-tool approval settings:
     ```toml
     [mcp_servers.docs.tools.search]
     approval_mode = "approve"
     ```
   - Plan mode reasoning effort override
   - SQLite state database location
   - Custom CA certificate support for enterprise environments
   - Experimental realtime start instructions

3. **MCP Support** (dual-mode):
   - **MCP Client**: Connects to external MCP servers for tool access
   - **MCP Server**: Experimental mode (`codex mcp-server`) that allows other agents to use Codex as a tool

4. **Sandbox Architecture** -- Platform-specific isolation:
   - **macOS**: `/usr/bin/sandbox-exec` with Seatbelt profiles
   - **Linux**: Landlock or Bubblewrap (`bwrap`) backends
   - **Windows**: Elevated/unelevated restricted-token backends
   - Three policy levels: `read-only` (default), `workspace-write`, `danger-full-access`

5. **Notification Hooks**: "Codex can run a notification hook when the agent finishes a turn." Includes client identification in payloads.

6. **Apps & Connectors**: ChatGPT connectors accessible via `$` in the composer; `/apps` command lists available and installed applications.

7. **Execution Modes**:
   - Interactive TUI (Ratatui-based terminal UI)
   - `codex exec` for headless/non-interactive automation
   - `--ephemeral` for temporary sessions without disk persistence
   - Desktop app and web-based agent at chatgpt.com/codex

8. **Multi-Platform IDE Integration**: Extensions for VS Code, Cursor, and Windsurf.

### Notable Extensions

- **MCP Server mode** -- Unique in the ecosystem: Codex can serve as an MCP server itself, enabling other tools to use it as a sub-agent
- **Platform-specific sandboxing** -- Most sophisticated sandbox implementation across all tools reviewed
- **GitHub**: https://github.com/openai/codex -- 74.5k stars, 10.5k forks, primarily Rust (94.9%)

### Patterns Worth Adopting

- **Dual MCP mode (client + server)**: Being both an MCP client and server enables composability -- Codex can be used as a tool by other agents, creating agent-of-agents architectures.
- **Per-tool approval modes in MCP config**: Fine-grained control over which MCP tools require user approval vs auto-approve. Critical for security in enterprise settings.
- **Platform-native sandboxing**: Using OS-level isolation (Seatbelt, Landlock, Bubblewrap) rather than containers for lightweight, secure execution.
- **Headless execution mode**: `codex exec` for CI/CD integration and automation without interactive UI.
- **Notification hooks on turn completion**: Enables integration with external notification systems (Slack, email, desktop notifications).
- **AGENTS.md as a convention**: Following Claude's CLAUDE.md pattern, establishing a standard for repository-level AI agent instructions.
- **Ephemeral sessions**: Temporary execution without persisting state, useful for one-off tasks and CI environments.

---

## Cross-Cutting Patterns

These patterns appear across multiple tools and represent emerging standards in the AI coding agent ecosystem:

### 1. Instruction Files (Universal)
Every tool has adopted some form of repository-level instruction file:
- Cursor: `.cursor/rules/*.mdc`
- Windsurf: Global/workspace rules + memories
- Aider: `CONVENTIONS.md` loaded via `--read`
- Continue: `.continue/rules/*.md`
- Cline: `.clinerules`
- Codex: `AGENTS.md`
- Claude Code: `CLAUDE.md`

**Pattern**: Markdown files with optional YAML frontmatter, version-controlled in the repository, loaded automatically.

### 2. Model Context Protocol (MCP) as the Extension Standard
MCP has become the de facto standard for tool extensibility:
- All six tools support MCP in some form
- Configuration is converging on JSON/YAML files in project directories
- Continue explicitly supports cross-tool MCP config compatibility
- 84.5k-star awesome-mcp-servers ecosystem with 45+ categories
- Codex goes further with dual client+server MCP mode

**Pattern**: MCP is the "USB port" of AI coding tools -- a universal connector for external capabilities.

### 3. Glob-Based Rule Targeting
Multiple tools support activating rules only for files matching specific patterns:
- Cursor: Glob patterns in rule frontmatter
- Continue: `globs` and `regex` properties on rules
- Aider: Language-specific lint commands via `--lint "language: cmd"`

**Pattern**: Context-aware rules that only activate when relevant, reducing noise.

### 4. Lint-Test-Fix Loops
Automatic quality gates after AI edits:
- Aider: `--auto-lint` + `--auto-test` with automatic fix attempts
- Windsurf: Auto-fix linting enabled by default
- Cline: Monitors linter/compiler errors and proactively fixes
- Continue: Integrated with VS Code problems panel

**Pattern**: Edit -> Lint -> Fix -> Test -> Fix cycle as an autonomous loop.

### 5. Checkpoint/Snapshot Systems
Safe experimentation through state snapshots:
- Windsurf: Named checkpoints with reversion
- Cline: Workspace state snapshots with diff comparison
- Aider: Git-based undo via `/undo` command

**Pattern**: Version-controlled experimentation points for safe AI-driven exploration.

### 6. Multi-Model/Multi-Provider Architecture
All tools support multiple LLM providers and model selection:
- Continue: Six distinct model roles (chat, autocomplete, edit, apply, embeddings, rerank)
- Aider: Primary + weak + editor models
- Cline: 10+ API providers + local models
- Codex: ChatGPT integration + API key modes

**Pattern**: Different models for different tasks based on capability and cost.

### 7. Community Sharing Platforms
Dedicated platforms for sharing configurations:
- Cursor: cursor.directory (web hub with Open Plugins standard)
- Aider: GitHub conventions repository with structured contribution
- Continue: Hub/Mission Control for centralized config management
- Cline: Built-in MCP marketplace

**Pattern**: Moving from individual config files to shared, discoverable ecosystems.

### 8. Headless/Automation Modes
Non-interactive execution for CI/CD:
- Aider: `--message` flag for single-task execution, Python API
- Codex: `codex exec` headless mode, `--ephemeral` sessions
- Continue: `cn` CLI for CI-enforceable AI checks

**Pattern**: AI coding agents as CI/CD pipeline steps, not just interactive tools.

### 9. IDE-Agnostic Design Choices
Tools increasingly support multiple environments:
- Codex: CLI + VS Code + Cursor + Windsurf + Desktop + Web
- Continue: VS Code + JetBrains + CLI
- Aider: Terminal + Browser UI + IDE watch mode
- Cline: VS Code + JetBrains

**Pattern**: Core logic separated from UI, enabling multiple frontends.

### 10. Conversational Extension Creation
AI-driven creation of extensions:
- Cline: "Add a tool that..." creates MCP servers via natural language
- Cursor: Skills as packaged AI capabilities

**Pattern**: The extension system itself is extensible via natural language, lowering the barrier to creating new tools.

---

## Sources

### Cursor
- https://github.com/pontusab/cursor-directory -- cursor.directory community hub (13k+ stars)
- https://github.com/PatrickJS/awesome-cursorrules -- Curated .cursorrules collection
- https://cursor.com/docs -- Official documentation

### Windsurf (Codeium)
- https://docs.windsurf.com/windsurf/cascade -- Cascade agent documentation
- https://docs.windsurf.com/windsurf/getting-started -- Getting started guide

### Aider
- https://github.com/Aider-AI/aider -- Main repository (43.1k stars)
- https://github.com/Aider-AI/conventions -- Community conventions repository (189 stars)
- https://aider.chat/docs/config.html -- Configuration documentation
- https://aider.chat/docs/usage/conventions.html -- Conventions documentation
- https://aider.chat/docs/usage/watch.html -- Watch mode documentation
- https://aider.chat/docs/usage/lint-test.html -- Lint/test integration
- https://aider.chat/docs/scripting.html -- Scripting and automation
- https://aider.chat/docs/git.html -- Git integration
- https://aider.chat/docs/config/adv-model-settings.html -- Advanced model settings

### Continue.dev
- https://github.com/continuedev/continue -- Main repository
- https://docs.continue.dev/customize/overview -- Customization overview
- https://docs.continue.dev/customize/model-roles -- Model roles
- https://docs.continue.dev/customize/deep-dives/rules -- Rules system
- https://docs.continue.dev/customize/deep-dives/mcp -- MCP integration
- https://github.com/continuedev/continue/tree/main/core/context/providers -- Context providers
- https://github.com/continuedev/continue/blob/main/core/tools/index.ts -- Tool architecture

### Cline
- https://github.com/cline/cline -- Main repository (60.1k stars)
- https://github.com/saoudrizwan/claude-dev -- Original Claude Dev repository
- https://github.com/cline/cline/tree/main/src/services/mcp -- MCP service architecture

### Codex CLI (OpenAI)
- https://github.com/openai/codex -- Main repository (74.5k stars)
- https://github.com/openai/codex/blob/main/AGENTS.md -- Agent instruction file
- https://github.com/openai/codex/blob/main/codex-rs/README.md -- Rust core architecture
- https://github.com/openai/codex/blob/main/docs/config.md -- Configuration guide
- https://github.com/openai/codex/blob/main/codex-rs/core/README.md -- Core sandbox architecture

### MCP Ecosystem
- https://github.com/punkpeye/awesome-mcp-servers -- MCP servers directory (84.5k stars, 45+ categories)
