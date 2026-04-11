# OpenClaw -- Extension Ecosystem Analysis

## Overview

OpenClaw (formerly Clawdbot, then Moltbot) is an open-source, self-hosted personal AI assistant framework created by Austrian developer Peter Steinberger (founder of PSPDFKit). It launched in November 2025 and went viral in late January 2026 after trademark disputes with Anthropic forced two rebrands. It is MIT-licensed and written in TypeScript, running on Node.js 20+.

At its core, OpenClaw is a long-running Node.js Gateway service that connects LLMs (Anthropic, OpenAI, Google, local models via Ollama, and 20+ other providers) to your local machine, messaging apps (WhatsApp, Telegram, Slack, Discord, Signal, iMessage, Teams, Matrix, IRC, LINE, and more), and a growing ecosystem of tools. It bridges "Natural Language Space" (user chat) to "Code Entity Space" (tool execution, file system access, system commands) through a unified runtime.

The extensibility model is layered into three primary mechanisms: **Skills** (markdown-driven prompt injection), **Plugins** (TypeScript runtime modules), and **Hooks** (event-driven automation scripts). On top of these, OpenClaw supports **MCP** (Model Context Protocol) servers for standards-based tool integration and **Lobster** for deterministic YAML-based workflow orchestration. A public registry called **ClawHub** hosts 13,000+ community-built skills.

## Key Extension Mechanisms

### 1. Skills (SKILL.md)

Skills are markdown files injected into the system prompt. They give the agent context, constraints, and step-by-step guidance for using tools effectively. Instead of embedding all tool instructions in every prompt (token-expensive), OpenClaw lists skills as metadata and lets the model read them on demand -- analogous to a developer looking up documentation as needed.

**Resolution priority:**
1. Workspace skills (`<workspace>/.openclaw/skills/`) -- highest
2. Managed skills (`~/.openclaw/skills/`) -- user-installed, shared across workspaces
3. Bundled skills (`<openclaw>/dist/skills/bundled/`) -- shipped with OpenClaw

Skills can be installed from ClawHub via `openclaw skills install` or written locally. They are the easiest extension point, requiring only a markdown file.

### 2. Plugins (TypeScript Runtime Modules)

Plugins are TypeScript modules loaded at runtime via `jiti`. They extend OpenClaw with new capabilities: channels, model providers, tools, speech, image generation, video generation, web fetch, web search, and more.

**Key characteristics:**
- Plugins export a `register(api)` function that calls methods like `api.registerTool`
- Agent tools are JSON-schema functions the LLM can call during a run
- Tools with side effects can be marked optional (never auto-enabled)
- Tool access is controlled via `tools.allow` / `tools.deny` config (deny always wins)
- Config validation uses the plugin manifest and JSON Schema without executing plugin code
- Plugins run in-process with the Gateway (treat as trusted code)
- Plugins can ship their own skills and hooks
- Bundled plugins must be explicitly enabled; installed plugins are enabled by default

**Bundled provider plugins:** Anthropic, Google, OpenAI, OpenRouter, GitHub Copilot, Mistral, Qwen, NVIDIA, HuggingFace, Cloudflare AI Gateway, and many more.

### 3. Hooks (Event-Driven Automation)

Hooks are small TypeScript functions that run when events fire inside the Gateway. They are automatically discovered from directories.

**Two kinds:**
- **Internal hooks:** Run inside the Gateway on agent events (`/new`, `/reset`, `/stop`, lifecycle events)
- **Webhooks:** External HTTP endpoints that let other systems trigger work in OpenClaw

**Discovery order (precedence):**
1. Workspace hooks (`<workspace>/hooks/`)
2. Managed hooks (`~/.openclaw/hooks/`)
3. Bundled hooks (`<openclaw>/dist/hooks/bundled/`)

**Bundled hooks:**
- `boot-md` -- Run BOOT.md on gateway startup
- `bootstrap-extra-files` -- Inject extra workspace bootstrap files during agent bootstrap
- `command-logger` -- Log all command events to a centralized audit file
- `session-memory` -- Save session context to memory on `/new` command

**Hook Packs** are standard npm packages that export hooks via `openclaw.hooks` in `package.json`.

Each hook directory needs a `HOOK.md` file (metadata: name, description, events, requirements) and a `handler.ts` file.

### 4. MCP (Model Context Protocol) Integration

OpenClaw has native MCP server support using `@modelcontextprotocol/sdk`. It acts as both:

- **MCP Host:** Connects to external MCP servers so agents can call their tools transparently
- **MCP Server (Bridge Mode):** `openclaw mcp serve` starts a stdio MCP server that exposes routed channel conversations over MCP

MCP servers are configured in `openclaw.json` by specifying server name, command, args, and transport (`stdio`, `sse`, or `streamable-http`). Tools appear alongside built-in tools.

**Multi-instance support:** A single MCP bridge can orchestrate multiple OpenClaw gateways (prod, staging, dev) with per-instance auth, timeout, and URL.

### 5. Lobster Workflow Engine

Lobster is OpenClaw's built-in deterministic workflow orchestration engine. It turns skills/tools into composable pipelines defined in YAML.

**Key design principles:**
- LLMs do creative work (writing code, analyzing, testing); YAML handles plumbing (sequencing, counting, routing, retrying)
- Workflows run in-process (no external CLI subprocess)
- Approval gates with resume tokens (halted workflows return a token; approve and resume without re-running)
- Timeouts, output caps, sandbox checks, and allowlists enforced by runtime
- Steps pass data between them as JSON

**Collaboration modes:** Orchestrator, Peer-to-Peer, and Hierarchical agent teams.

### 6. Webhook-Driven TaskFlows (v2026.4.7)

The latest addition that closes the loop between external events and agent execution:
- External services push events via webhook endpoints
- A Webhook Session Key (`hook:<uuid>`) ties incoming requests to a specific agent session
- Enables patterns like: GitHub PR opened -> agent reviews code -> posts comments (within seconds, no polling)
- Reduces latency by up to 90% vs. polling and significantly lowers API token costs

### 7. ClawFlows (Prebuilt Workflow Library)

ClawFlows sits on top of the agent runtime as a structured library of YAML workflow definitions. Each file contains name, description, trigger configuration (scheduled, event-driven, or manual), skill mappings, parameter defaults, and output format. Categories: communication, developer, finance, smart home, health.

A ClawFlows agent can generate workflows from plain English descriptions, create PRs with the YAML config, and let you review before deploying.

## Notable Extensions / Integrations

### Memory
- **memory-lancedb** -- Vector-backed long-term memory with auto-recall/capture using LanceDB
- **memU** -- Memory framework for 24/7 proactive agents with lower token cost
- **MemOS Cloud** -- Cloud memory for persistent recall
- **supermemory** -- Long-term memory extension
- **memory-wiki** (v2026.4.7) -- Persistent knowledge storage as wiki pages

### Security
- **SecureClaw** -- Skill + plugin for on-demand auditing and continuous runtime security monitoring
- **AI Security Scanner** -- Open-source scanner for agent skill packages
- **ToxicSkills audit** found 36.82% of ClawHub skills had security flaws

### Developer Tools
- **Claude Code Plugin** -- Turns Claude Code CLI into a programmable, headless coding engine
- **Composio Plugin** -- Single plugin providing access to 850+ SaaS apps via managed MCP server with OAuth
- **OpenClaw Foundry** -- Self-modification: observes workflows, writes new skills/extensions/hooks, validates in sandbox before deploying

### Voice & Communication
- **Voice Call plugin** -- Twilio integration for making/receiving phone calls
- **Channel plugins** for Teams, Matrix, Nostr, QQ, Feishu/Lark, DingTalk

### Dashboards
- **ClawX** -- Desktop GUI
- **clawdeck** -- Mission-control dashboard
- **openclaw-studio** -- Polished web dashboard
- **GMClaw Dashboard** -- Visual interface for agent orchestration

### Infrastructure
- **MCP Server for OpenClaw** -- Secure bridge between Claude.ai and self-hosted OpenClaw with OAuth2
- **ROS Bridge** -- Universal ROS1/ROS2 bridge for robot control
- **Bitcoin Wallet** -- Bitcoin wallet for AI agents with hardware enclave key storage

## Patterns Worth Adopting

### 1. Three-Layer Extension Architecture (Skills / Plugins / Hooks)

**What OpenClaw does:** Separates extensions into three complexity tiers -- markdown-based prompt injection (skills), runtime code modules (plugins), and event-driven scripts (hooks).

**Compozy mapping:** Compozy already has skills (bundled markdown files). Consider formalizing a three-tier model:
- **Skills** (existing) -- markdown prompt injection, zero code
- **Extensions** (existing, being built) -- Go modules with typed registration
- **Hooks** -- lightweight event-driven scripts that fire on lifecycle events (task start, task complete, PR opened, review requested, build failed)

### 2. Deterministic Workflow Engine Separate from LLM Reasoning

**What OpenClaw does:** Lobster lets you define multi-step pipelines in YAML where LLMs handle creative work and the workflow engine handles sequencing, routing, retrying. This yields determinism, auditability, and resumability.

**Compozy mapping:** Compozy's kernel dispatcher and plan system already do some of this. A formal YAML/TOML workflow definition layer could let users define custom pipelines (e.g., "run PRD -> techspec -> task breakdown -> execute" with conditional branches, approval gates, and resume tokens). This would make Compozy workflows diffable, versionable, and shareable.

### 3. Workspace-Scoped Extension Discovery with Precedence

**What OpenClaw does:** Skills, hooks, and plugins are discovered from workspace-local, user-global, and bundled directories in that precedence order. This lets projects override global behavior.

**Compozy mapping:** Support `.compozy/extensions/` in the workspace for project-specific extensions that override user-global (`~/.compozy/extensions/`) and bundled ones. This is critical for team-shared extension configurations.

### 4. Webhook/Event-Driven Task Triggering

**What OpenClaw does:** External services push events via webhooks that trigger agent sessions. GitHub PR -> auto-review, CI failure -> root cause analysis, PagerDuty alert -> incident runbook.

**Compozy mapping:** A webhook ingress extension that lets CI/CD pipelines trigger Compozy workflows. For example: GitHub webhook on PR open -> Compozy runs review workflow -> posts comments. Or: CI failure webhook -> Compozy analyzes logs and suggests fixes.

### 5. Tool Registration via JSON Schema

**What OpenClaw does:** Agent tools are JSON-schema-typed functions registered through plugins. Tools can be marked optional (never auto-enabled for safety). Access is controlled via allow/deny lists where deny always wins.

**Compozy mapping:** Extension-provided tools could follow the same pattern -- each extension registers typed tool definitions with JSON Schema, and Compozy's kernel routes tool calls. An allow/deny config prevents dangerous tools from auto-activating.

### 6. Self-Modifying Extension Generator (Foundry Pattern)

**What OpenClaw does:** OpenClaw Foundry observes your workflows, researches docs, and writes new skills/extensions/hooks. It validates generated code in a sandbox before deploying.

**Compozy mapping:** A "compozy generate extension" command that analyzes a user's workflow patterns, identifies automation opportunities, and scaffolds new extensions. The extension could generate Go extension code, validate it compiles, and register it.

### 7. Approval Gates with Resume Tokens

**What OpenClaw does:** Lobster workflows can pause at approval gates and return a resume token. Approving with the token continues execution without re-running previous steps.

**Compozy mapping:** For long-running multi-task workflows, Compozy could support checkpoints. If a human needs to approve a techspec before task generation begins, the workflow pauses, saves state, and resumes on approval. This is especially valuable for review remediation workflows.

### 8. ClawFlows-Style Prebuilt Workflow Library

**What OpenClaw does:** 111+ prebuilt workflows covering communication, developer, finance, smart home, and health domains. Users customize via YAML override files, leaving source workflow files untouched.

**Compozy mapping:** A curated library of workflow templates for common dev patterns: "greenfield feature", "bug triage", "dependency upgrade", "refactoring campaign", "security audit", "onboarding new contributor". Users select a template and override specific steps.

### 9. Session Branching and Restore

**What OpenClaw does:** v2026.4.7 adds session branch/restore for reproducible AI runs -- fork a session, experiment, and restore if needed.

**Compozy mapping:** Git worktree-based session branching is a natural fit. Compozy could formalize "branch a run" (create a worktree, run experimental tasks, merge or discard). This aligns with the existing worktree support.

### 10. Multi-Instance Bridge Architecture

**What OpenClaw does:** A single MCP bridge can orchestrate multiple OpenClaw gateways (prod, staging, dev) with per-instance auth and routing.

**Compozy mapping:** For teams, a Compozy coordinator that routes tasks to different agent instances based on task type, priority, or environment. A "task router" extension that distributes work across multiple agent backends.

## Sources

- [OpenClaw Official Site](https://openclaw.ai/)
- [OpenClaw GitHub Repository](https://github.com/openclaw/openclaw)
- [OpenClaw Plugins Documentation](https://docs.openclaw.ai/tools/plugin)
- [OpenClaw Hooks Documentation](https://docs.openclaw.ai/automation/hooks)
- [OpenClaw MCP Documentation](https://docs.openclaw.ai/cli/mcp)
- [Lobster Workflow Engine (GitHub)](https://github.com/openclaw/lobster)
- [Lobster Documentation](https://docs.openclaw.ai/tools/lobster)
- [ClawFlows: 111 Prebuilt AI Workflows (SitePoint)](https://www.sitepoint.com/clawflows-prebuilt-ai-workflows-openclaw/)
- [OpenClaw Ecosystem Map 2026](https://openclawnews.online/article/openclaw-ecosystem-2026)
- [Top OpenClaw Integrations (DEV Community)](https://dev.to/composiodev/top-openclaw-integrations-to-connect-your-workflow-in-2026-1l5h)
- [OpenClaw v2026.4.7 Release Notes](https://blockchain.news/ainews/openclaw-v2026-4-7-release-gemma-4-ollama-vision-webhook-taskflows-and-memory-wiki-latest-ai-workflow-breakthrough)
- [Awesome OpenClaw (vincentkoc)](https://github.com/vincentkoc/awesome-openclaw)
- [Awesome OpenClaw Skills (VoltAgent)](https://github.com/VoltAgent/awesome-openclaw-skills)
- [Top 10 OpenClaw Plugins (Composio)](https://composio.dev/content/top-openclaw-plugins)
- [OpenClaw Reference Architecture (RobotPaper)](https://robotpaper.ai/reference-architecture-openclaw-early-feb-2026-edition-opus-4-6/)
- [Building Multi-Agent Pipelines with Lobster (DEV Community)](https://dev.to/ggondim/how-i-built-a-deterministic-multi-agent-dev-pipeline-inside-openclaw-and-contributed-a-missing-4ool)
- [OpenClaw Hooks Event-Driven Guide (Meta Intelligence)](https://www.meta-intelligence.tech/en/insight-openclaw-hooks-guide)
- [OpenClaw MCP Integration Guide](https://computertech.co/openclaw-mcp-integration-guide-2026/)
- [OpenClaw Multi-Agent Orchestration Guide](https://zenvanriel.com/ai-engineer-blog/openclaw-multi-agent-orchestration-guide/)
- [OpenClaw Claude Code Plugin (GitHub)](https://github.com/Enderfga/openclaw-claude-code)
- [Run OpenClaw in VS Code via ACP (DEV Community)](https://dev.to/formulahendry/run-openclaw-in-vs-code-through-acp-agent-client-protocol-4amo)
- [OpenClaw Webhooks Guide (Skywork)](https://skywork.ai/skypage/en/openclaw-webhooks-ai-automation/2038597463222075392)
- [OpenClaw Extensions Marketplace 2026](https://openclawblog.space/articles/openclaw-extensions-marketplace-best-community-plugins-2026)
- [Best OpenClaw Skills 2026 (ClawdHost)](https://clawdhost.net/blog/best-openclaw-skills-2026/)
- [OpenClaw Security (ToxicSkills Audit)](https://managemyclaw.com/blog/openclaw-ecosystem-tools-map-2026/)
