# Pi-Mono -- Extension Ecosystem Analysis

## Overview

Pi-Mono is an open-source AI agent toolkit created by Mario Zechner (badlogic, creator of libGDX), hosted at [github.com/badlogic/pi-mono](https://github.com/badlogic/pi-mono). It is a TypeScript monorepo (~23.9k GitHub stars as of April 2026) containing packages that layer on top of each other to provide a complete coding agent system. The primary artifact is **pi**, a minimal terminal coding harness that ships with only four default tools (read, write, edit, bash) and relies on its extension system to add everything else.

The official site is [shittycodingagent.ai](https://shittycodingagent.ai/), and the npm package is published as [@mariozechner/pi-coding-agent](https://www.npmjs.com/package/@mariozechner/pi-coding-agent).

### Core Philosophy

Pi is "aggressively extensible so it doesn't have to dictate your workflow." Features that other tools (Claude Code, Cursor, Aider) bake in -- such as sub-agents, plan mode, permission popups, MCP support, and to-do tracking -- are intentionally omitted from the core and pushed to extensions. This keeps the core minimal while letting users shape pi to fit their workflow using TypeScript extensions, skills, prompt templates, and themes.

### Monorepo Packages

| Package           | Purpose                                                                                                                           |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `pi-ai`           | Unified LLM API across Anthropic, OpenAI, Google, xAI, Groq, Cerebras, OpenRouter, Ollama, 300+ models                            |
| `pi-agent-core`   | Stateful agent runtime with tool execution, event streaming, conversation management                                              |
| `pi-coding-agent` | Full interactive coding agent CLI with built-in file tools, session persistence, context compaction, skills, and extension system |
| `pi-tui`          | Minimal terminal UI framework with differential rendering                                                                         |
| `pi-web-ui`       | Web components for AI chat interfaces                                                                                             |
| `pi-mom`          | Slack bot for message delegation                                                                                                  |
| `pi-pods`         | CLI for managing vLLM deployments on GPU pods                                                                                     |

## Key Extension Mechanisms

Pi provides four primary extensibility layers, plus an SDK and RPC mode for programmatic embedding.

### 1. Extensions (TypeScript Modules)

Extensions are the core extensibility primitive. They are TypeScript modules (previously called "hooks") that enhance pi's behavior by handling lifecycle events, registering custom tools, adding commands, and providing UI components.

**Discovery:**

- Global: `~/.pi/agent/extensions/*.ts`
- Project-local: `.pi/extensions/*.ts`
- Explicit: `pi -e ./path.ts` (for quick tests)
- Via packages (npm or git)

**Basic structure:**

```typescript
import type { ExtensionAPI } from "@mariozechner/pi-coding-agent";

export default function (pi: ExtensionAPI) {
  pi.on("event_name", async (event, ctx) => { ... });
  pi.registerTool({ name, label, description, parameters, execute });
  pi.registerCommand("name", { ... });
  pi.registerShortcut("ctrl+x", { ... });
  pi.registerFlag("my-flag", { ... });
}
```

**Key capabilities:**

- **Custom tools** -- `pi.registerTool()` registers tools callable by the LLM, using TypeBox schemas for parameters. Tools can be registered at load time or dynamically inside event handlers. New tools are refreshed immediately in the same session.
- **Tool overrides** -- Extensions can override built-in tools (read, bash, edit, write, grep, find, ls) by registering a tool with the same name. Interactive mode displays a warning when this happens.
- **Event interception** -- Block or modify tool calls, inject context, customize compaction.
- **User interaction** -- Prompt users via `ctx.ui` (select, confirm, input, notify).
- **Custom UI components** -- Full TUI components with keyboard input via `ctx.ui.custom()`.
- **Custom commands** -- Register commands like `/mycommand` via `pi.registerCommand()`.
- **Session persistence** -- Store state that survives restarts via `pi.appendEntry()`.
- **Hot reload** -- Extensions in auto-discovered locations can be hot-reloaded with `/reload`.

**Lifecycle events:**

| Event                                                       | Description                                                         |
| ----------------------------------------------------------- | ------------------------------------------------------------------- |
| `session_start`                                             | Session begins                                                      |
| `session_shutdown`                                          | Session ends                                                        |
| `session_before_compact`                                    | Customize summarization before context compaction                   |
| `session_before_fork` / `session_fork`                      | Session forking                                                     |
| `session_before_tree` / `session_tree`                      | Session tree operations                                             |
| `context`                                                   | Rewrite messages before the LLM sees them (RAG, memory, filtering)  |
| `tool_call`                                                 | Intercept or gate tool invocations (e.g., block dangerous commands) |
| `tool_result`                                               | Modify tool results before they reach the LLM                       |
| `input`                                                     | User input events                                                   |
| `resources_discover`                                        | Resource discovery (with reason "reload" on hot reload)             |
| `agent_start`                                               | Agent starts (used in SDK inline extensions)                        |
| `bash` / `read` / `edit` / `write` / `grep` / `find` / `ls` | Typed per-tool events with specific event interfaces                |

### 2. Skills (Capability Packages)

Skills are Markdown-based capability packages with instructions and tools, loaded on-demand. They provide progressive disclosure without busting the prompt cache.

**Discovery locations:**

- Global: `~/.pi/agent/skills/`
- Project: `.pi/skills/`
- Explicit: `--skill path/to/skill.md`

**Invocation:** Available as `/skill:name` from the prompt. The model can also autonomously load a skill when the task matches its description.

Skills are similar to Compozy's existing skill system.

### 3. Prompt Templates (Markdown Snippets)

Prompt templates are Markdown snippets that expand into full prompts. The user types `/name` in the editor to invoke a template.

**Features:**

- Positional argument substitution (`$1`, `$2`, `$3`, ...)
- All arguments (`$@` or `$ARGUMENTS`)
- Argument slicing (`${@:N}` or `${@:N:L}`)
- Autocomplete with descriptions

**Locations:** `~/.pi/agent/prompts/`, `.pi/prompts/`, or via packages.

### 4. Themes (Visual Customization)

Themes customize the TUI appearance. Distributable via packages.

### 5. Packages (Distribution Mechanism)

Packages bundle extensions, skills, prompt templates, and themes for distribution. They can be installed from npm or git:

```
pi install npm:@foo/pi-tools
pi install git:github.com/badlogic/pi-doom
```

Packages are managed with `pi update`, `pi list`, and `pi config`. A community package registry is available at [shittycodingagent.ai/packages](https://shittycodingagent.ai/packages).

### 6. SDK and RPC Mode

**SDK:** Programmatic access via `createAgentSession()` -- embed pi in other applications, build custom interfaces, or integrate with automated workflows. Supports multi-session runtimes.

**RPC Mode:** Headless operation via JSON protocol over stdin/stdout (strict JSONL). Useful for embedding in IDEs or custom UIs. Extension UI interactions (select, confirm, input) are translated into a request/response sub-protocol.

**Operational Modes:** Interactive (full TUI), print/JSON (single-shot), RPC (JSON protocol), SDK (programmatic embedding).

### 7. System Prompt Customization

- Replace default system prompt with `.pi/SYSTEM.md` (project) or `~/.pi/agent/SYSTEM.md` (global)
- Append without replacing via `APPEND_SYSTEM.md`
- Project instructions via `AGENTS.md` files (loaded at startup from `~/.pi/agent/`, parent directories, and cwd)

## Notable Extensions / Integrations

### From the awesome-pi-agent List

| Extension                 | Author     | Description                                                                                |
| ------------------------- | ---------- | ------------------------------------------------------------------------------------------ |
| **pi-notify**             | ferologics | Native desktop notifications via OSC 777                                                   |
| **pi-ghostty-theme-sync** | -          | Sync Ghostty terminal theme with pi session                                                |
| **pi-sketch**             | -          | Quick sketch pad: draw in browser, send to models                                          |
| **pi-dcp**                | -          | Dynamic context pruning for intelligent conversation optimization                          |
| **pi-gui**                | -          | GUI extension providing visual interface for pi agent                                      |
| **pi-super-curl**         | -          | Empower curl requests with coding agent capabilities                                       |
| **cost-tracker**          | -          | Session spending analysis from pi logs                                                     |
| **handoff**               | -          | Transfer context to new focused sessions                                                   |
| **memory-mode**           | -          | Save instructions to AGENTS.md with AI-assisted integration                                |
| **filter-output**         | -          | Redact sensitive data (API keys, tokens, passwords) from tool results before LLM sees them |
| **security**              | -          | Block dangerous bash commands and protect sensitive paths from writes                      |
| **pi-canvas**             | -          | Interactive TUI canvases (calendar, document, flights) rendered inline                     |
| **pi-sub**                | -          | Usage tracking extensions with shared core and UI widget                                   |
| **pi-rewind-hook**        | -          | Rewind file changes with git-based checkpoints and conversation branching                  |
| **pi-ssh-remote**         | -          | Redirect all file operations and commands to a remote host via SSH                         |
| **plan-mode**             | -          | Read-only exploration mode for safe code exploration                                       |
| **oracle**                | -          | Get second opinion from alternative AI models without switching contexts                   |
| **pi-mcp-adapter**        | -          | MCP (Model Context Protocol) support as an extension                                       |
| **safe-git**              | -          | Safe git operations                                                                        |
| **pi-cost-dashboard**     | -          | Cost monitoring dashboard                                                                  |
| **checkpoint**            | -          | Checkpoint and restore                                                                     |

### Popular Community Picks (from curated list)

pi-mcp-adapter, safe-git, pi-cost-dashboard, pi-notify, checkpoint, pi-canvas

### Emerging Extensions (2026 PRs)

pi-aliyun (provider), pi-elixir, pi-secret-guard, pipane, pi-governance, meyhem-search (skill), pi-rewind, pi-memory

### Extension Collections

- **tmustier/pi-extensions** -- In-terminal file browser/viewer, tab-status indicators for parallel sessions, usage statistics dashboard, minigames (Space Invaders, Pacman, Pong, Tetris, Mario)
- **jayshah5696/pi-agent-extensions** -- Sessions management, ask_user interaction, handoff

### oh-my-pi (omp) -- Notable Fork

[oh-my-pi](https://github.com/can1357/oh-my-pi) by Can Boluk is a batteries-included fork that adds features pi deliberately omits:

| Feature                            | Description                                                                                                                                                                                                                                    |
| ---------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Hash-anchored edits (Hashline)** | Every line gets a content-hash anchor; the model references anchors instead of reproducing text. Eliminates "string not found" and ambiguous match errors. If the file changed, hashes won't match and the edit is rejected before corruption. |
| **LSP integration**                | 11 LSP operations (diagnostics, definition, references, hover, symbols, rename, code_actions, etc.), format-on-write, diagnostics-on-edit, 40+ language configs out of the box                                                                 |
| **Subagents**                      | Spawn child agents for parallelizable subtasks with results merged back. Isolation via git worktrees, Unix fuse-overlay, or Windows ProjFS. Async background jobs with configurable concurrency (up to 100).                                   |
| **Browser control**                | Built-in browser automation                                                                                                                                                                                                                    |
| **Python kernel**                  | Persistent kernel with rich output (HTML, Markdown, images, Mermaid diagrams)                                                                                                                                                                  |
| **Custom modules**                 | Loadable from `.omp/modules/` and `~/.omp/agent/modules/`                                                                                                                                                                                      |

## Patterns Worth Adopting

### 1. Typed Lifecycle Event System

Pi's extension system is built around typed lifecycle events (`session_start`, `tool_call`, `tool_result`, `context`, `session_before_compact`, etc.) with typed event interfaces for each built-in tool. Extensions subscribe to events and can intercept, modify, or block operations.

**Compozy mapping:** Compozy's dispatcher/chain system already has event dispatching. Adopting typed per-tool events (e.g., `BeforePlanEvent`, `AfterRunEvent`, `BeforePromptEvent`) with the ability to intercept and modify payloads would give extensions fine-grained control over the pipeline.

### 2. Dynamic Tool Registration

Extensions can register new LLM-callable tools at any point during the lifecycle -- at load time, inside event handlers, or after session start. Tools are refreshed immediately. Extensions can also override built-in tools.

**Compozy mapping:** Allow extensions to register custom kernel commands/handlers dynamically. An extension could add a new `/deploy` command or override the default plan behavior.

### 3. Context Injection and Filtering

The `context` event lets extensions rewrite messages before the LLM sees them. This enables RAG pipelines, long-term memory injection, sensitive data filtering, and custom context augmentation -- all without modifying the core.

**Compozy mapping:** Add a `ContextTransform` hook in the prompt pipeline where extensions can inject retrieved context, filter sensitive information, or augment prompts with project-specific knowledge.

### 4. Progressive Skill Loading

Skills are loaded on-demand rather than stuffed into the system prompt. This preserves prompt cache efficiency while making capabilities available when needed. The model can also autonomously decide to load a skill.

**Compozy mapping:** Compozy already has skills. The autonomous loading pattern (model decides when to load based on task description) could be adopted to reduce upfront prompt size.

### 5. Package Distribution via npm/git

Pi packages bundle extensions, skills, prompts, and themes. They are installable from npm or git with simple commands. A community registry provides discoverability.

**Compozy mapping:** A `compozy install` command that pulls extension packages from Go modules, git repos, or a registry. Extensions could be Go plugins, WASM modules, or script-based (TypeScript via Compozy's existing SDK).

### 6. Session Persistence and Checkpointing

Extensions can persist state across restarts via `pi.appendEntry()`. The pi-rewind extension adds git-based checkpoints with conversation branching. The checkpoint extension adds save/restore.

**Compozy mapping:** Compozy's journal system already handles durable events. Exposing checkpoint/restore to extensions would let them implement rollback workflows (e.g., revert a failed multi-task execution).

### 7. Security Extensions (filter-output, security)

Community extensions that redact sensitive data from tool output before the LLM sees it, and block dangerous bash commands or protect sensitive paths. These are not built into the core -- they are opt-in extensions.

**Compozy mapping:** A `SecurityPolicy` extension type that can intercept agent commands before execution, redact secrets from prompts, and enforce path restrictions. This is especially relevant since Compozy orchestrates multiple AI agents.

### 8. Cost Tracking and Observability

Multiple community extensions (cost-tracker, pi-cost-dashboard, pi-sub) provide session spending analysis, usage statistics, and cost monitoring.

**Compozy mapping:** An extension that hooks into Compozy's run events to track token usage, cost per task, cost per agent, and cost trends across runs. Expose as a dashboard or report.

### 9. Multi-Agent Orchestration (from oh-my-pi)

omp's subagent system spawns child agents for parallelizable subtasks with isolation via git worktrees and merge strategies (patch or branch). Async background jobs with configurable concurrency.

**Compozy mapping:** Compozy already does multi-agent orchestration. The git worktree isolation pattern and merge strategies from omp could enhance Compozy's parallel task execution, especially for review remediation workflows.

### 10. Hash-Anchored Edits (from oh-my-pi)

Hashline gives every line a content-hash anchor. The model references anchors instead of reproducing text. Eliminates ambiguous matches and detects stale reads.

**Compozy mapping:** Could be exposed as an extension that wraps the edit tool for agents that support it, reducing edit failures in long coding sessions.

### 11. RPC Mode for Embedding

Pi's RPC mode enables headless operation via JSON protocol over stdio, with extension UI interactions translated into a request/response sub-protocol.

**Compozy mapping:** An RPC/IPC interface would allow Compozy to be embedded in IDEs, CI/CD pipelines, or custom dashboards. The extension UI sub-protocol pattern is worth studying for interactive extensions in headless environments.

### 12. Oracle / Second Opinion Pattern

The oracle extension lets users get a second opinion from an alternative AI model without switching contexts.

**Compozy mapping:** A `review-with` extension that sends completed task output to a different model for review, catching issues the primary agent missed. Useful for Compozy's PR review workflows.

## Sources

- [Pi-Mono GitHub Repository](https://github.com/badlogic/pi-mono)
- [Pi-Mono README](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/README.md)
- [Pi Extensions Documentation](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/extensions.md)
- [Pi Packages Documentation](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/packages.md)
- [Pi Prompt Templates Documentation](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/prompt-templates.md)
- [Pi SDK Documentation](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/sdk.md)
- [Pi RPC Documentation](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/rpc.md)
- [Pi Extension Types Source](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/extensions/types.ts)
- [Pi Official Site](https://shittycodingagent.ai/)
- [Pi Packages Registry](https://shittycodingagent.ai/packages)
- [awesome-pi-agent Community List](https://github.com/qualisero/awesome-pi-agent)
- [oh-my-pi (omp) Fork](https://github.com/can1357/oh-my-pi)
- [Pi DeepWiki Overview](https://deepwiki.com/badlogic/pi-mono)
- [Pi Extensibility System (Mintlify)](https://www.mintlify.com/badlogic/pi-mono/concepts/extensibility)
- [Mario Zechner Blog Post on Pi](https://mariozechner.at/posts/2025-11-30-pi-coding-agent/)
- [Pi-Mono Explained (hoangyell)](https://hoangyell.com/pi-mono-explained/)
- [Pi-Mono Medium Article](https://ai-engineering-trend.medium.com/pi-mono-the-minimalist-ai-coding-assistant-behind-openclaw-bd3ccc0a1b04)
- [Building a Music Agent with Pi-Mono](https://www.mager.co/blog/2026-02-16-beatbrain-chat-pi-mono)
- [Custom Agent Framework with PI (Nader Dabit)](https://nader.substack.com/p/how-to-build-a-custom-agent-framework)
- [py-pimono Python Port](https://github.com/solvit-team/py-pimono)
- [@mariozechner/pi-coding-agent on npm](https://www.npmjs.com/package/@mariozechner/pi-coding-agent)
