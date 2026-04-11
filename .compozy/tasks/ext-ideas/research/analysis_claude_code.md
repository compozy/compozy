# Claude Code -- Extension Ecosystem Analysis

## Overview

Claude Code (Anthropic's agentic CLI) has evolved from a terminal coding assistant into a comprehensive extensibility platform with **five distinct extension mechanisms**: CLAUDE.md (advisory context), Skills (on-demand workflows via slash commands), Hooks (deterministic lifecycle automation), MCP Servers (external tool/data connections), and Plugins (packaging layer that bundles all of the above). As of early 2026, the ecosystem includes 3,000+ indexed MCP servers (mcp.so), 400+ community-built servers, and a growing marketplace of plugins, skills, and agent configurations.

Claude Code also provides the **Claude Agent SDK** (`@anthropic-ai/claude-agent-sdk`) for programmatic embedding -- the same agent loop and tools that power Claude Code, available as TypeScript and Python packages. Headless mode (`claude -p`) enables CI/CD integration and scripting without interactive UI.

---

## Key Extension Mechanisms

### 1. CLAUDE.md -- Advisory Context

- A markdown file at the project root that Claude reads automatically on session start.
- Contains coding conventions, bash commands, architectural patterns, key file pointers.
- Scoping: `CLAUDE.md` (project, checked in), `CLAUDE.local.md` (personal, gitignored), `~/.claude/CLAUDE.md` (global).
- **Limitation**: Advisory only -- Claude follows instructions ~70% of the time. For rules that must be enforced 100%, hooks are required.
- Best practice: Keep under 200 lines / ~2,500 tokens. Longer files cause instruction adherence to drop measurably.

### 2. Skills -- Reusable Workflows & Slash Commands

- A `.md` file in `.claude/skills/` (project) or `~/.claude/skills/` (global).
- The directory name becomes the `/slash-command`. Example: `.claude/skills/deploy/SKILL.md` creates `/deploy`.
- YAML frontmatter with `description` enables automatic invocation when Claude deems it relevant.
- Can include templates, example files, and scripts alongside the SKILL.md entrypoint.
- Skills use progressive disclosure: ~100 tokens during metadata scan, <5k tokens when activated.
- Available to Pro, Max, Team, and Enterprise users. Not available on Free tier.

### 3. Hooks -- Deterministic Lifecycle Automation

Hooks are shell commands, LLM prompts, or subagents that execute automatically at specific points in Claude Code's lifecycle. Unlike CLAUDE.md instructions, hooks execute every time -- guaranteed.

**12 Hook Events:**

| Event | When it fires | Can block? |
|-------|--------------|-----------|
| `PreToolUse` | Before Claude performs an action (write file, run command) | Yes (exit 2) |
| `PostToolUse` | After Claude completes an action | Yes (blocks continuation) |
| `PostToolUseFailure` | When a tool execution fails | Yes |
| `UserPromptSubmit` | When user submits a prompt, before processing | Yes |
| `Stop` | When Claude tries to finish responding | Yes (exit 2 forces continuation) |
| `SubagentStop` | When a subagent tries to finish | Yes |
| `SubagentStart` | When a subagent starts | No |
| `Notification` | When Claude sends alerts (permission, idle, auth) | No |
| `PermissionRequest` | When Claude displays a permission dialog | Yes (approve/deny) |
| `SessionStart` | On session start, resume, clear, or compact | No |
| `SessionEnd` | On session exit, sigint, or error | No |
| `PreCompact` | Before compaction operation | No |

**Handler Types**: Command (shell), HTTP (POST), Prompt (LLM evaluation), Agent (deep analysis).

**Configuration**: `~/.claude/settings.json` (global) or `.claude/settings.json` (project, version-controlled).

**Common Use Cases**:
- Auto-format code on every file write (PostToolUse)
- Block writes to `.env` files (PreToolUse)
- Auto-create backup commits before big changes (PreToolUse)
- Run linter after every edit and feed output back (PostToolUse)
- Desktop notifications when Claude needs attention (Notification)
- Enforce test execution before declaring done (Stop)
- Security scanning: 9 built-in patterns (command injection, XSS, eval, pickle, os.system)

### 4. MCP Servers -- External Tool & Data Connections

MCP (Model Context Protocol) is an open-source standard for connecting AI models to external systems. Claude Code spawns MCP servers as separate processes and exposes their tools within its context.

**Scoping**: Local (current user, current project), Project (`.mcp.json`, shared via VCS), User (all projects).

**MCP Tool Search**: Lazy loading reduces context usage by up to 95%. Servers' tools only load when relevant.

**Top MCP Servers (by popularity):**

| Server | Purpose |
|--------|---------|
| GitHub MCP | Repos, issues, PRs, CI/CD without context-switching |
| PostgreSQL MCP | Query databases, inspect schemas, write migrations |
| Sentry MCP | Error tracking, pattern analysis, fix suggestions |
| Playwright MCP | Browser automation -- navigate, fill forms, screenshot |
| Memory MCP | Persistent knowledge graph across sessions |
| Filesystem MCP | Advanced glob, file watching, batch operations, metadata |
| Slack MCP | Read/send messages, bridge coding and communication |
| Notion MCP | Search/create pages, sync architecture docs |
| Brave Search MCP | Web search for current information |
| Apidog MCP | API spec access for accurate code generation |

**Building Your Own**: Anthropic's TypeScript SDK (`@modelcontextprotocol/sdk`) handles the protocol layer. A minimal MCP server is under 50 lines of code. Python SDK also available.

### 5. Plugins -- Packaging & Distribution

Plugins bundle skills, hooks, subagents, and MCP servers into a single installable unit. Installed via `/plugin` command (public beta).

**Plugin Structure:**
```
plugin-name/
  .claude-plugin/
    plugin.json       # Metadata
  commands/            # Slash commands
  agents/              # Specialized agents
  skills/              # Agent skills
  hooks/               # Event handlers
  .mcp.json            # MCP server config
  README.md
```

**Namespacing**: Plugin skills are namespaced (e.g., `/my-plugin:review`) so multiple plugins coexist.

**Marketplaces**: Any GitHub repo with a `.claude-plugin/marketplace.json` can serve as a marketplace. Browse and install via `/plugin marketplace add user-or-org/repo-name`.

**Security**: Plugin subagents do NOT support hooks, mcpServers, or permissionMode frontmatter. For those, agents must live in `.claude/agents/` outside the plugin package.

### 6. DXT/MCPB -- Desktop Extension Packaging

Desktop Extensions package MCP servers into single `.dxt` files (being renamed to `.mcpb`) for one-click installation. Analogous to Chrome `.crx` or VS Code `.vsix` files.

- Bundles all dependencies (Node.js built into Claude Desktop).
- `manifest.json` for metadata and configuration.
- Supports Node.js, Python, and binary MCP servers.
- Toolchain: `npm install -g @anthropic-ai/dxt`, `dxt init`, `dxt pack`.
- Open-sourced specification and toolchain.

### 7. Custom Agents / Subagents

- Define persistent specialist agents as `.md` files with YAML frontmatter in `.claude/agents/`.
- Built-in agents: Explore (read-only, Haiku), Plan (read-only), General (all tools).
- Custom agents can specify model, tool restrictions, system prompt, memory scope.
- `memory: user` lets an agent build knowledge over time across sessions.
- Agent Teams (experimental): Multiple Claude instances coordinate via shared task list with direct inter-agent communication.

### 8. Claude Agent SDK -- Programmatic Embedding

- npm: `@anthropic-ai/claude-agent-sdk` (TypeScript), `claude-agent-sdk` (Python).
- Core function `query()` returns an async iterator of messages (reasoning, tool calls, results).
- Same tools, agent loop, and context management as Claude Code.
- Supports `allowedTools`, `permissionMode`, `systemPrompt`, `mcpServers`, structured JSON output.
- Supports Anthropic API, AWS Bedrock, Google Vertex AI, Microsoft Azure Foundry.
- `--bare` mode skips OAuth/keychain reads for scripted/CI use.

---

## Notable Extensions / Integrations

### Curated Lists & Directories

| Repository | Scale |
|-----------|-------|
| [rohitg00/awesome-claude-code-toolkit](https://github.com/rohitg00/awesome-claude-code-toolkit) | 135 agents, 35 skills, 42 commands, 176+ plugins, 20 hooks |
| [hesreallyhim/awesome-claude-code](https://github.com/hesreallyhim/awesome-claude-code) | Skills, hooks, slash-commands, agent orchestrators |
| [jqueryscript/awesome-claude-code](https://github.com/jqueryscript/awesome-claude-code) | Broad tools, plugins, integrations, frameworks |
| [travisvn/awesome-claude-skills](https://github.com/travisvn/awesome-claude-skills) | Skills-focused directory |
| [ComposioHQ/awesome-claude-plugins](https://github.com/ComposioHQ/awesome-claude-plugins) | Plugin-focused curation |
| [quemsah/awesome-claude-plugins](https://github.com/quemsah/awesome-claude-plugins) | 220+ skills & agent plugins with adoption metrics |

### High-Impact Community Projects

**Multi-Agent Orchestration:**
- **Ralph for Claude Code** -- Autonomous AI development framework with intelligent exit detection, rate limiting, circuit breaker patterns, and safety guardrails. Enables Claude Code to work iteratively until completion.
- **maestro-orchestrate** -- Multi-agent orchestration coordinating 22 specialized subagents through 4-phase workflows with native parallel execution.
- **ruflo** (ruvnet/ruflo) -- Agent orchestration platform with distributed swarm intelligence, RAG integration, and native Claude Code / Codex integration.
- **AgentSys** (avifenesh) -- Workflow automation automating task-to-production workflows, PR management, code cleanup, drift detection, and multi-agent code review.

**Skills & Command Libraries:**
- **Anthropic official skills** (37.5k stars) -- Public repository for Agent Skills.
- **obra/superpowers** -- Core skills library with 20+ battle-tested skills including TDD, debugging, collaboration patterns; `/brainstorm`, `/write-plan`, `/execute-plan` commands.
- **Claude Command Suite** (qdhenry) -- 216+ slash commands, 12 skills, 54 agents, automated workflows for code review, testing, deployment.
- **wshobson/commands** -- 57 production-ready slash commands (15 workflows, 42 tools).
- **Trail of Bits skills** (1.3k stars) -- Security research, vulnerability detection, audit workflows.
- **claude-seo** (1.5k stars) -- Universal SEO analysis skill.
- **gsap-skills** (1.5k stars) -- Official GSAP animation platform integration.
- **owasp-security** -- OWASP Top 10:2025, ASVS 5.0, and Agentic AI security with code review checklists for 20+ languages.

**Context & Memory:**
- **MemClaw** -- Persistent project memory: stores architecture decisions, coding conventions, task progress, session history. Eliminates re-explanation at session start.
- **Memory MCP Server** -- Knowledge graph persistence across sessions (entities, relationships, observations).
- **context-mode** -- Processes large outputs in sandboxed subprocesses, keeping only summaries in context (98% context savings).

**Integration Plugins:**
- **connect-apps** -- Connects Claude to Gmail, Slack, GitHub, Notion, and 500+ services.
- **nano-banana** -- Google Gemini image generation (text-to-image, style transfer, 4K output).
- **OpenPaw** -- 38-skill bundle turning Claude Code into a personal assistant.

**Observability:**
- **claude-code-hooks-multi-agent-observability** (disler) -- Real-time monitoring for Claude Code agents through hook event tracking.
- **claude-code-hooks-mastery** (disler) -- Comprehensive hook examples and patterns.

---

## Patterns Worth Adopting

These patterns from the Claude Code ecosystem map directly to Compozy's architecture and could inform its extension system.

### Pattern 1: Deterministic Lifecycle Hooks

**Claude Code pattern**: 12 lifecycle events with shell/HTTP/prompt/agent handlers, exit-code-based blocking, JSON stdin/stdout protocol.

**Compozy mapping**: The `internal/core/run` execution pipeline and `internal/core/kernel` dispatcher could expose similar lifecycle hooks:
- `PreTaskExecution` / `PostTaskExecution` -- before/after each task in a batch
- `PreAgentInvocation` / `PostAgentInvocation` -- before/after spawning Claude Code, Codex, Droid, Cursor
- `PrePRCreation` / `PostPRCreation` -- before/after PR creation
- `OnReviewRemediation` -- when review feedback arrives
- `OnPlanGeneration` / `OnTaskBreakdown` -- during plan/spec/task phases
- Handler types: shell command, Go plugin, HTTP webhook

### Pattern 2: Skills as Markdown-First Extension Points

**Claude Code pattern**: `.claude/skills/` directories with SKILL.md entrypoints, YAML frontmatter for metadata, progressive disclosure (~100 tokens scan, <5k tokens activation).

**Compozy mapping**: The existing `skills/` directory already follows this pattern. Could be extended with:
- Registry of community skills installable via `compozy skill install <repo>`
- Skill auto-detection based on task domain (already partially implemented via Agent Skill Dispatch Protocol in CLAUDE.md)
- Skill versioning and dependency resolution

### Pattern 3: Plugin Packaging with Namespace Isolation

**Claude Code pattern**: `.claude-plugin/plugin.json` metadata, namespaced commands (`/plugin:command`), marketplace repos.

**Compozy mapping**: Compozy extensions could follow a similar structure:
```
compozy-ext-name/
  compozy-plugin.toml    # Metadata (matching Compozy's TOML preference)
  skills/                # Bundled skills
  hooks/                 # Lifecycle hooks
  templates/             # PRD/TechSpec/ADR templates
  agents/                # Agent configurations
```
Install via `compozy ext install <repo>`. Namespace: `compozy ext:skill-name`.

### Pattern 4: MCP Server Integration for External Context

**Claude Code pattern**: MCP servers provide tools and resources to Claude at runtime. Lazy loading via MCP Tool Search.

**Compozy mapping**: During task enrichment (`internal/core/plan`), Compozy could query MCP servers for:
- Codebase analysis results (architecture, dependency graphs)
- Issue tracker context (JIRA, Linear, GitHub Issues)
- Monitoring data (Sentry errors, performance metrics)
- Documentation (Notion, Confluence)
- This would enrich the prompt builders in `internal/core/prompt` with live external data.

### Pattern 5: Multi-Agent Orchestration Patterns

**Claude Code pattern**: Subagents via Task tool (up to 10 parallel), Agent Teams (experimental, inter-agent communication), hierarchical orchestration, split-and-merge for independent subtasks.

**Compozy mapping**: The `internal/core/run` pipeline already orchestrates multiple agents. Could adopt:
- Agent specialization profiles (`.compozy/agents/`) defining model, tools, system prompt per agent type
- Split-and-merge for independent tasks in a batch (already partially there)
- Hierarchical orchestration: lead agent decomposes feature into sub-tasks, each delegated to specialized sub-agents
- `COMPOZY_SUBAGENT_MODEL` environment variable for cost optimization

### Pattern 6: Persistent Memory Across Sessions

**Claude Code pattern**: Memory MCP server (knowledge graph), MemClaw (project-scoped memory), `memory: user` agent frontmatter, AGENTS.md accumulating patterns.

**Compozy mapping**: The `internal/core/run/journal` already provides durable event journals. Could extend with:
- Cross-run knowledge accumulation (what patterns worked, what failed)
- Architecture decision cache (from `.compozy/tasks/` ADRs)
- Agent performance metrics per codebase (which agent handles which task types best)

### Pattern 7: Headless/CI Integration with Structured Output

**Claude Code pattern**: `claude -p` for non-interactive use, `--output-format stream-json`, `--bare` for credential isolation, tool permission presets.

**Compozy mapping**: Compozy already supports CI via CLI. Could adopt:
- Structured JSON output for pipeline integration (`compozy run --output json`)
- Event streaming for real-time dashboard integration
- Pre-configured permission profiles for CI environments

### Pattern 8: Quality Enforcement via Hook Chains

**Claude Code pattern**: PostToolUse hooks auto-format and lint. Stop hooks verify tests pass before declaring done. PreToolUse hooks block dangerous operations.

**Compozy mapping**: The `make verify` gate is already enforced via CLAUDE.md instructions. Could be hardened with:
- `PostTaskExecution` hook running `make verify` automatically
- `PrePRCreation` hook ensuring all checks pass
- `OnAgentOutput` hook validating generated code before writing
- Security scanning hooks (similar to Claude Code's 9 security patterns)

---

## Gaps & Pain Points That Extensions Could Address

1. **No persistent memory** -- Agents forget everything between sessions. A memory/knowledge-graph extension solves this.
2. **Context window exhaustion** -- Large codebases fill context fast. Subagent delegation and context-mode patterns help.
3. **Rate limits and cost** -- Power users hit limits quickly. Model routing (Opus for planning, Sonnet/Haiku for execution) and caching reduce costs.
4. **Quality drift in long sessions** -- "Rush to completion" behavior increases with session length. Verification hooks enforce quality gates.
5. **No native web access** -- Agents cannot check latest docs or APIs. Search MCP servers fill this gap.
6. **Manual quality enforcement** -- CLAUDE.md rules get ignored. Hooks provide 100% enforcement.
7. **IDE fragmentation** -- Limited editor support. SDK embedding enables custom integrations.
8. **Trust-then-verify gap** -- Agents produce plausible but incorrect code. Automated test execution and verification hooks address this.

---

## Sources

### Official Documentation
- [Extend Claude Code - Features Overview](https://code.claude.com/docs/en/features-overview)
- [Hooks Reference](https://code.claude.com/docs/en/hooks)
- [Skills Documentation](https://code.claude.com/docs/en/skills)
- [MCP Server Configuration](https://code.claude.com/docs/en/mcp)
- [Agent Teams](https://code.claude.com/docs/en/agent-teams)
- [Headless/Programmatic Usage](https://code.claude.com/docs/en/headless)
- [Agent SDK Overview](https://platform.claude.com/docs/en/agent-sdk/overview)
- [Best Practices](https://code.claude.com/docs/en/best-practices)
- [Claude Code Plugins Blog Post](https://claude.com/blog/claude-code-plugins)
- [Plugins README (GitHub)](https://github.com/anthropics/claude-code/blob/main/plugins/README.md)
- [Enabling Claude Code Autonomous Work](https://www.anthropic.com/news/enabling-claude-code-to-work-more-autonomously)
- [Desktop Extensions Engineering Blog](https://www.anthropic.com/engineering/desktop-extensions)

### Community & Ecosystem
- [awesome-claude-code-toolkit](https://github.com/rohitg00/awesome-claude-code-toolkit) -- 135 agents, 35 skills, 176+ plugins
- [awesome-claude-code (hesreallyhim)](https://github.com/hesreallyhim/awesome-claude-code)
- [awesome-claude-code (jqueryscript)](https://github.com/jqueryscript/awesome-claude-code)
- [awesome-claude-skills (travisvn)](https://github.com/travisvn/awesome-claude-skills)
- [awesome-claude-plugins (ComposioHQ)](https://github.com/ComposioHQ/awesome-claude-plugins)
- [awesome-claude-plugins (quemsah)](https://github.com/quemsah/awesome-claude-plugins) -- adoption metrics
- [awesome-dxt-mcp](https://github.com/MCPStar/awesome-dxt-mcp)
- [Claude Command Suite](https://github.com/qdhenry/Claude-Command-Suite)
- [wshobson/commands](https://github.com/wshobson/commands) -- 57 production-ready slash commands
- [wshobson/agents](https://github.com/wshobson/agents) -- multi-agent orchestration
- [disler/claude-code-hooks-mastery](https://github.com/disler/claude-code-hooks-mastery)
- [disler/claude-code-hooks-multi-agent-observability](https://github.com/disler/claude-code-hooks-multi-agent-observability)
- [ruflo agent orchestration platform](https://github.com/ruvnet/ruflo)
- [DXT toolchain (Anthropic)](https://github.com/anthropics/dxt)

### SDKs & Packages
- [@anthropic-ai/claude-agent-sdk (npm)](https://www.npmjs.com/package/@anthropic-ai/claude-agent-sdk)
- [@anthropic-ai/claude-code (npm)](https://www.npmjs.com/package/@anthropic-ai/claude-code)
- [claude-agent-sdk-typescript (GitHub)](https://github.com/anthropics/claude-agent-sdk-typescript)
- [claude-agent-sdk-python (GitHub)](https://github.com/anthropics/claude-agent-sdk-python)
- [ClaudeCodeSDK Swift (jamesrochabrun)](https://github.com/jamesrochabrun/ClaudeCodeSDK)
- [@modelcontextprotocol/sdk](https://modelcontextprotocol.io/docs/develop/connect-local-servers)

### Analysis & Guides
- [Claude Code Extensions Guide (Morph)](https://www.morphllm.com/claude-code-extensions)
- [Claude Code Hooks Guide (Morph)](https://www.morphllm.com/claude-code-hooks)
- [Claude Code Hooks Guide (eesel AI)](https://www.eesel.ai/blog/hooks-in-claude-code)
- [Claude Code Hooks Guide (DataCamp)](https://www.datacamp.com/tutorial/claude-code-hooks)
- [Claude Code Hooks Guide (DEV Community)](https://dev.to/lukaszfryc/claude-code-hooks-complete-guide-with-20-ready-to-use-examples-2026-dcg)
- [All 12 Hook Events Reference (Pixelmojo)](https://www.pixelmojo.io/blogs/claude-code-hooks-production-quality-ci-cd-patterns)
- [All 12 Hook Events (claudefast)](https://claudefa.st/blog/tools/hooks/hooks-guide)
- [Claude Code Customization Guide (alexop.dev)](https://alexop.dev/posts/claude-code-customization-guide-claudemd-skills-subagents/)
- [Claude Code CLI Reference (blakecrosley)](https://blakecrosley.com/guides/claude-code)
- [Best MCP Servers for Claude Code (apidog)](https://apidog.com/blog/top-10-mcp-servers-for-claude-code/)
- [Best MCP Servers for Claude Code (MCPcat)](https://mcpcat.io/guides/best-mcp-servers-for-claude-code/)
- [Skills vs MCP Servers (DEV Community)](https://dev.to/williamwangai/claude-code-skills-vs-mcp-servers-what-to-use-how-to-install-and-the-best-ones-in-2026-548k)
- [Sub-Agent Best Practices (claudefast)](https://claudefa.st/blog/guide/agents/sub-agent-best-practices)
- [Multi-Agent Orchestration (shipyard)](https://shipyard.build/blog/claude-code-multi-agent/)
- [Code Agent Orchestra (Addy Osmani)](https://addyosmani.com/blog/code-agent-orchestra/)
- [Claude Code Pain Points Q1 2026 (DEV Community)](https://dev.to/shuicici/claude-codes-feb-mar-2026-updates-quietly-broke-complex-engineering-heres-the-technical-5b4h)
- [Claude Code Economics (Medium)](https://medium.com/@william.couturier/claude-code-in-march-2026-the-economics-of-the-quota-792449b63edb)
- [How I Use Claude Code (builder.io)](https://www.builder.io/blog/claude-code)
- [Claude Code Best Practices (sidetool)](https://www.sidetool.co/post/claude-code-best-practices-tips-power-users-2025)
- [Advanced Claude Code Workflows (sidetool)](https://www.sidetool.co/post/advanced-claude-code-workflows-tips-and-tricks-for-power-users/)
- [Claude Code for Advanced Users (cuttlesoft)](https://cuttlesoft.com/blog/2026/02/03/claude-code-for-advanced-users/)
- [13 MCP Server Integrations (DEV Community)](https://dev.to/wedgemethoddev/how-i-built-13-mcp-server-integrations-for-claude-code-with-source-code-4pk4)
- [Plugins Guide (skillsplayground)](https://skillsplayground.com/guides/claude-code-plugins/)
- [Plugins Guide (felo.ai)](https://felo.ai/blog/claude-code-plugins-guide/)
- [Building Agents with Claude Agent SDK (nader)](https://nader.substack.com/p/the-complete-guide-to-building-agents)
