# Hermes Agent -- Extension Ecosystem Analysis

## Overview

Hermes Agent is an open-source, self-improving AI agent built by Nous Research, released in February 2026 under the MIT license. As of April 2026, it has 32k+ GitHub stars, 142+ contributors, and 2,293+ commits. It is written in Python.

Unlike IDE-tethered coding copilots, Hermes is a persistent autonomous agent that lives on your server, remembers what it learns across sessions, and compounds in capability over time. It runs on any infrastructure (local, Docker, SSH, Daytona, Modal, Singularity) and communicates over 15+ messaging platforms (Telegram, Discord, Slack, WhatsApp, Signal, Matrix, etc.) or via terminal TUI.

The core innovation is a **four-stage learning loop**: (1) execute tasks using 47+ built-in tools, (2) evaluate outcomes through explicit/implicit feedback, (3) abstract successful patterns into reusable skill documents, (4) refine those skills during future use. After 10-20 similar tasks, execution speed improves 2-3x.

### Key differentiators from other AI coding agents

- **Self-improving skills**: Skills are created and refined by the agent itself, not by humans.
- **Persistent memory**: SQLite FTS5 with ~10ms search latency across 10k+ entries; three-tier system (session, persistent, skill memory).
- **Model-agnostic**: Works with OpenRouter (200+ models), Anthropic, OpenAI, Google, MiniMax, and any OpenAI-compatible endpoint.
- **Sub-agents**: Isolated subagents with their own conversations, terminals, and Python RPC scripts for zero-context-cost pipelines.
- **Natural-language cron**: Schedule recurring tasks in plain English; results delivered to any messaging platform.

---

## Key Extension Mechanisms

Hermes provides four distinct extensibility layers:

### 1. Plugin System

Plugins are the primary extension mechanism. A plugin is a directory with four files:

```
~/.hermes/plugins/my-plugin/
  plugin.yaml      # manifest: name, version, description, provides_tools, provides_hooks, requires_env
  __init__.py      # register(ctx) function -- wires schemas to handlers
  schemas.py       # tool schemas (JSON Schema definitions visible to the LLM)
  tools.py         # tool handlers (execution logic)
```

**Plugin discovery** from three sources:

- User-global: `~/.hermes/plugins/`
- Project-local: `.hermes/plugins/` (requires `HERMES_ENABLE_PROJECT_PLUGINS=true`)
- pip entry points: auto-discovered from installed Python packages

**Plugin types** with different activation rules:

- General plugins: multi-select (enable/disable any combination)
- Memory providers: single-select (one active at a time)
- Context engines: single-select (one active at a time)

**Registration API** -- the `register(ctx)` function receives a context object with:

| Method                                                       | Purpose                               |
| ------------------------------------------------------------ | ------------------------------------- |
| `ctx.register_tool(name, schema, handler)`                   | Register a tool the LLM can call      |
| `ctx.register_hook(event_name, callback)`                    | Attach to lifecycle events            |
| `ctx.register_cli_command(name, help, setup_fn, handler_fn)` | Add CLI subcommands                   |
| `ctx.inject_message(content, role)`                          | Queue a message into the conversation |

**Conditional tool availability**: Tools can be gated with a `check_fn` lambda that hides the tool from the LLM if prerequisites are missing.

**Environment variable gating**: `requires_env` in plugin.yaml declares API keys with descriptions and signup URLs. Missing variables disable the plugin gracefully.

**Distribution**: Plugins can be distributed via pip packages using `pyproject.toml` entry points under `hermes_agent.plugins`, or installed from GitHub repos via `hermes plugins install user/repo`.

### 2. Lifecycle Event Hooks

Six lifecycle hooks allow plugins to intercept the agent loop:

| Hook               | Fires When                | Callback Args                        |
| ------------------ | ------------------------- | ------------------------------------ |
| `pre_tool_call`    | Before tool execution     | `tool_name, args, task_id`           |
| `post_tool_call`   | After tool execution      | `tool_name, args, result, task_id`   |
| `pre_llm_call`     | Before each LLM API call  | Returns context injection dict       |
| `post_llm_call`    | After each LLM completion | Turn data                            |
| `on_session_start` | New session begins        | `session_id, model, platform`        |
| `on_session_end`   | Session ends              | `session_id, completed, interrupted` |

Key design decisions:

- `pre_llm_call` injects context into the user message (not system prompt) to preserve prompt cache efficiency.
- Hook crashes are logged and skipped; other hooks and the agent continue normally.
- Hooks accept `**kwargs` for forward compatibility.
- `post_tool_call` fires for ALL tool calls, not just the registering plugin's own tools.

### 3. Skills System (agentskills.io Standard)

Skills are structured markdown files stored in `~/.hermes/skills/` that follow the open agentskills.io standard (originally created by Anthropic, December 2025). The same SKILL.md format works across Claude Code, OpenAI Codex, Gemini CLI, GitHub Copilot, Cursor, VS Code, and 20+ other platforms.

**Progressive disclosure** minimizes token usage:

- Level 0: List of skill names/descriptions (~3,000 tokens)
- Level 1: Full content of a specific skill
- Level 2: Specific reference file within a skill

**Autonomous skill creation**: After complex tasks (5+ tool calls), the agent synthesizes experience into permanent skill documents. Every 15 tasks, the agent evaluates overall performance. Skills evolve as newer approaches prove better.

**Skill format**: YAML front matter for metadata + markdown body for instructions. Name constraints: lowercase letters, numbers, hyphens only; max 64 characters.

### 4. MCP (Model Context Protocol) Integration

Hermes connects to any MCP server via stdio or HTTP transport, providing access to external tools (GitHub, databases, file systems, APIs) without writing native Hermes tools. Features include:

- Per-server tool filtering
- Sampling support
- OAuth 2.1 PKCE authentication
- OSV malware scanning of MCP extension packages

Additionally, Hermes can expose **itself** as an MCP server (v0.6.0+), allowing IDEs and other tools to use Hermes as a backend.

---

## Notable Extensions / Integrations

### Official Integrations

| Category         | Integrations                                                                                                                                                                                    |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| AI Providers     | OpenRouter (200+), Anthropic, OpenAI, Google Gemini, MiniMax, z.ai/GLM, Kimi/Moonshot, any OpenAI-compatible                                                                                    |
| Messaging        | Telegram, Discord, Slack, WhatsApp, Signal, Matrix, Mattermost, Email, SMS, DingTalk, Feishu/Lark, WeCom, Weixin, BlueBubbles, Home Assistant                                                   |
| Memory Providers | Honcho (dialectic reasoning), OpenViking (tiered retrieval), Mem0 (cloud extraction), Hindsight (knowledge graphs), Holographic (local SQLite), RetainDB (hybrid search), ByteRover (CLI-based) |
| Web Search       | Firecrawl (default), Parallel, Tavily, Exa                                                                                                                                                      |
| Browser          | Browserbase (cloud), Browser Use, local Chrome CDP, headless Chromium                                                                                                                           |
| Voice/TTS        | Edge TTS, ElevenLabs, OpenAI TTS, MiniMax, NeuTTS; STT: Whisper, Groq, OpenAI                                                                                                                   |
| IDE              | ACP-compatible editors: VS Code, Zed, JetBrains                                                                                                                                                 |
| API Server       | OpenAI-compatible HTTP endpoint for Open WebUI, LobeChat, LibreChat, NextChat, ChatBox                                                                                                          |

### Community Plugins (from evey/hermes-plugins -- 23 plugins)

**Autonomy and Decision-Making:**

- **evey-autonomy**: Autonomous decision-making, planning, and reflection
- **evey-council**: Multi-model consensus through structured debate
- **evey-delegate-model**: Intelligent task routing with retry logic and sensitivity filtering

**Observability and Telemetry:**

- **evey-telemetry**: Structured JSON logging of every tool call, delegation, and error
- **evey-status**: Unified status aggregation across bridge, MQTT, costs, cron, and goals
- **evey-mqtt**: Real-time event streaming with auto-subscription
- **evey-cost-guard**: Budget enforcement via Langfuse with progressive alerts

**Quality and Safety:**

- **evey-reflect**: Self-correction via critique from lightweight models
- **evey-validate**: Hallucination detection using regex + LLM scoring
- **evey-email-guard**: Prompt injection screening (20+ regex patterns + local AI classifier)
- **evey-sandbox**: Resource-limited code execution environment
- **evey-session-guard**: Session lifecycle management

**Learning and Memory:**

- **evey-learner**: Experience-based learning with knowledge application
- **evey-memory-adaptive**: Importance-weighted memory with decay mechanisms
- **evey-memory-consolidate**: Nightly consolidation into MEMORY.md and vector storage
- **evey-cache**: Delegation result caching to avoid redundant queries
- **evey-identity**: Self-updating SOUL.md personality evolution

**Communication and Integration:**

- **evey-bridge**: File-based bidirectional communication with Claude Code
- **evey-goals**: Autonomous goal lifecycle management
- **evey-research**: Web search and note-taking pipeline
- **evey-digest**: Multi-source daily compilation
- **evey-delegation-score**: Quality ranking of delegation outcomes

### Other Notable Community Projects

- **hermes-workspace** (500+ stars): Web-based GUI with chat, terminal, memory browser, skills manager, and inspector
- **mission-control** (3.7k+ stars): Open-source dashboard for AI agent fleet orchestration, task dispatch, cost tracking
- **wondelai/skills** (380+ stars): Cross-platform skills library compatible with Hermes and other agents
- **hermes-agent-self-evolution**: DSPy + GEPA framework for evolutionary optimization of skills and prompts
- **hermes-payguard**: Safe USDC/x402 payment plugin with spending limits and approval flows
- **hermes-web-search-plus**: Multi-provider web search with intelligent routing (Serper, Tavily, Exa)
- **hermes-paperclip-adapter**: Integration with Paperclip task management

---

## Patterns Worth Adopting

The following patterns from Hermes Agent's ecosystem could inspire Compozy extensions:

### 1. Lifecycle Hook System for Extensions

**Hermes pattern**: Six well-defined lifecycle hooks (`pre_tool_call`, `post_tool_call`, `pre_llm_call`, `post_llm_call`, `on_session_start`, `on_session_end`) that plugins register against.

**Compozy mapping**: Compozy's `internal/core/extension/dispatcher.go` already has a chain/dispatcher pattern. Consider formalizing hook points around the task execution lifecycle:

- `pre_task_execute` / `post_task_execute`
- `pre_agent_dispatch` / `post_agent_dispatch`
- `on_run_start` / `on_run_end`
- `pre_pr_review` / `post_pr_review`

Hermes's design choice to inject hook context into the user message rather than the system prompt (for cache efficiency) is worth noting.

### 2. Plugin Manifest with Declarative Capabilities

**Hermes pattern**: `plugin.yaml` declares what a plugin provides (`provides_tools`, `provides_hooks`, `requires_env`) and the system does capability-based activation. Environment variable gating disables plugins gracefully when prerequisites are missing.

**Compozy mapping**: Extensions could declare their capabilities in a TOML manifest:

```toml
[extension]
name = "cost-tracker"
version = "1.0.0"
provides_hooks = ["post_task_execute"]
requires_env = ["LANGFUSE_API_KEY"]
```

### 3. Conditional Tool/Extension Availability

**Hermes pattern**: Tools use `check_fn` lambdas to hide themselves from the LLM when prerequisites are missing. This prevents the agent from attempting to use tools that cannot work.

**Compozy mapping**: Extensions could register enablement predicates. For example, a GitHub integration extension would only activate when git remotes are configured and `gh` CLI is authenticated.

### 4. Cost Guard and Budget Enforcement

**Hermes pattern** (evey-cost-guard): Progressive budget alerts via Langfuse, enforcing spending limits per session/cron job with escalating warnings.

**Compozy mapping**: A Compozy extension that tracks token usage across agent runs, enforces per-task/per-batch budgets, and emits warnings through the event journal. Could integrate with the existing `pkg/compozy/events` envelope system.

### 5. Multi-Model Consensus / Council Pattern

**Hermes pattern** (evey-council): Send the same task to multiple models and use structured debate to reach consensus, improving decision quality for high-stakes changes.

**Compozy mapping**: For PR reviews or architecture decisions, dispatch the same review to multiple agent backends (Claude, GPT, Gemini) and synthesize a consensus opinion. Maps to the existing `internal/core/agent` validator pattern.

### 6. Delegation Scoring and Quality Ranking

**Hermes pattern** (evey-delegation-score): Rank the quality of delegated task outcomes to learn which agent/model combinations work best for different task types.

**Compozy mapping**: Track which agent IDE (Claude Code, Codex, Cursor, Droid) produces the best results for different task categories (feature, bugfix, refactor, test). Use this data to optimize future agent selection in `internal/core/agent`.

### 7. Natural-Language Cron Scheduling

**Hermes pattern**: Schedule recurring agent tasks with plain English ("every weekday at 8am, check CI status"). Jobs run in isolated sessions, with delivery to messaging platforms.

**Compozy mapping**: A Compozy extension for scheduled workflows: "every morning, run the review batch and post a summary to Slack." Could build on the existing `internal/core/run/journal` for durable scheduling.

### 8. Observability and Telemetry Extension

**Hermes pattern** (evey-telemetry + evey-status): Structured JSON logging of every tool call, delegation, and error. Unified status aggregation across subsystems.

**Compozy mapping**: An extension that emits structured telemetry events from the run pipeline, tracking task duration, agent success/failure rates, token usage, and cost. Feeds into dashboards or alerting systems. Natural fit with `pkg/compozy/events`.

### 9. Sub-Agent Isolation with Context Boundaries

**Hermes pattern**: Subagents run in isolated processes with their own conversation contexts and terminals. No context bleed to the parent agent. Python RPC scripts collapse multi-step pipelines into zero-context-cost turns.

**Compozy mapping**: When executing large task batches, spawn isolated agent processes per task with clear context boundaries. The existing `internal/core/run` pipeline could benefit from stronger isolation guarantees between parallel task executions.

### 10. Self-Improving Skill Creation

**Hermes pattern**: After complex tasks (5+ tool calls), the agent automatically synthesizes reusable skill documents. Skills evolve as newer approaches prove better. Every 15 tasks, the agent self-evaluates.

**Compozy mapping**: After completing execution batches, an extension could analyze patterns in successful task executions and generate refined skill documents for future runs. The existing `skills/` directory in Compozy already uses the agentskills.io standard, making this a natural fit.

### 11. Cross-Agent Bridge

**Hermes pattern** (evey-bridge): File-based bidirectional communication between Hermes and Claude Code, allowing context sharing and task handoff between different agents.

**Compozy mapping**: Compozy already orchestrates multiple agent types. A formalized bridge protocol could enable richer context passing between agents -- for example, having Claude Code's output feed directly into Codex for review, with shared context artifacts.

### 12. Memory Consolidation

**Hermes pattern** (evey-memory-consolidate): Nightly batch consolidation of conversation fragments into structured MEMORY.md files and vector storage, with importance-weighted decay.

**Compozy mapping**: An extension that consolidates learnings from completed task batches into project-level knowledge bases. Over multiple development cycles, this builds institutional memory about the codebase: which areas are fragile, which patterns work best, common review feedback themes.

---

## Comparison: Hermes vs. Other AI Coding Agents on Extensibility

| Feature          | Hermes Agent                          | Claude Code                          | Aider                                | Mentat                  | SWE-agent               |
| ---------------- | ------------------------------------- | ------------------------------------ | ------------------------------------ | ----------------------- | ----------------------- |
| Plugin system    | Full (plugin.yaml + Python)           | Full (plugins marketplace, Oct 2025) | None (3rd-party editor plugins only) | None (VS Code ext only) | PR proposed, not merged |
| Lifecycle hooks  | 6 hooks (pre/post tool, LLM, session) | Pre/post-tool hooks, HTTP hooks      | None                                 | None                    | None                    |
| Skills/SKILL.md  | Yes (agentskills.io standard)         | Yes (created the standard)           | No                                   | No                      | No                      |
| MCP support      | Full (stdio + HTTP + OAuth)           | Full                                 | No                                   | No                      | No                      |
| Sub-agents       | Isolated with Python RPC              | Worktrees + subagents                | No                                   | No                      | No                      |
| Scheduled tasks  | Natural-language cron                 | No                                   | No                                   | No                      | No                      |
| Memory providers | 7 pluggable backends                  | Built-in                             | No                                   | RAG auto-context        | No                      |
| IDE integration  | ACP (VS Code, Zed, JetBrains)         | VS Code, JetBrains                   | 3rd-party VS Code, JetBrains, Nova   | VS Code                 | Browser UI              |
| Self-improvement | Built-in learning loop                | No                                   | No                                   | No                      | No                      |

---

## Sources

- [Hermes Agent GitHub Repository](https://github.com/NousResearch/hermes-agent)
- [Hermes Agent Official Documentation](https://hermes-agent.nousresearch.com/docs/)
- [Hermes Agent Plugin System Docs](https://hermes-agent.nousresearch.com/docs/user-guide/features/plugins/)
- [Build a Hermes Plugin Guide](https://hermes-agent.nousresearch.com/docs/guides/build-a-hermes-plugin/)
- [Hermes Agent Architecture](https://hermes-agent.nousresearch.com/docs/developer-guide/architecture/)
- [Hermes Agent Integrations](https://hermes-agent.nousresearch.com/docs/integrations/)
- [Hermes Agent Cron Scheduling](https://hermes-agent.nousresearch.com/docs/user-guide/features/cron)
- [Hermes Agent Releases](https://github.com/NousResearch/hermes-agent/releases)
- [Awesome Hermes Agent](https://github.com/0xNyk/awesome-hermes-agent)
- [Evey's Hermes Plugins (23 plugins)](https://github.com/42-evey/hermes-plugins)
- [Hermes Agent v0.8.0 Background Tasks](https://juliangoldie.com/hermes-agent-v0-8-0/)
- [Hermes Agent Self-Evolution](https://github.com/NousResearch/hermes-agent-self-evolution)
- [agentskills.io Specification](https://agentskills.io/specification)
- [Agent Skills GitHub Repository](https://github.com/agentskills/agentskills)
- [DEV.to: Hermes Agent Self-Improving AI](https://dev.to/arshtechpro/hermes-agent-a-self-improving-ai-agent-that-runs-anywhere-2b7d)
- [AI.cc: Hermes Agent 2026](https://www.ai.cc/blogs/hermes-agent-2026-self-improving-open-source-ai-agent-vs-openclaw-guide/)
- [The New Stack: Persistent AI Agents Compared](https://thenewstack.io/persistent-ai-agents-compared/)
- [Hindsight Memory Integration](https://hindsight.vectorize.io/sdks/integrations/hermes)
- [MindStudio: Hermes Agent Overview](https://www.mindstudio.ai/blog/what-is-hermes-agent-openclaw-alternative)
- [Hooks, Plugins, and Sessions in AI Coding Agents (ToLearn Blog)](https://tolearn.blog/blog/2026-04-02-hooks-plugins-sessions-ai-agents)
