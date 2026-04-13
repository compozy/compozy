# Reusable Agents

Reusable agents let you package a prompt, runtime defaults, and optional agent-local MCP servers into a directory that Compozy can discover and execute.

`compozy setup` also provisions the built-in council advisor roster globally under `~/.compozy/agents/`:

- `architect-advisor`
- `devils-advocate`
- `pragmatic-engineer`
- `product-mind`
- `security-advocate`
- `the-thinker`

Those bundled council agents intentionally inherit the host runtime, which keeps council debates consistent across supported drivers.

## Discovery and Override Rules

Supported discovery scopes:

- workspace: `.compozy/agents/<name>/`
- global: `~/.compozy/agents/<name>/`

Rules:

- the directory name is the canonical agent id
- names must match `^[a-z][a-z0-9-]{0,63}$`
- `compozy` is reserved and cannot be used as an agent name
- when a workspace and global agent share the same name, the workspace directory wins as a whole
- invalid agent directories are reported per-agent, but they do not prevent other valid agents from loading

Supported v1 files inside an agent directory:

- `AGENT.md`
- optional `mcp.json`

Deferred fields and folders stay out of scope in v1:

- frontmatter fields `extends`, `uses`, `skills`, and `memory` are rejected
- sibling `skills/` and `memory/` directories are ignored

## `AGENT.md`

`AGENT.md` uses YAML frontmatter plus a markdown body. Compozy reads these frontmatter fields today:

| Field              | Purpose                                                                         |
| ------------------ | ------------------------------------------------------------------------------- |
| `title`            | Human-facing name shown in inspect output                                       |
| `description`      | Short description shown in list output and the prompt-visible discovery catalog |
| `ide`              | Default runtime ide for this agent                                              |
| `model`            | Default model override                                                          |
| `reasoning_effort` | Default reasoning effort (`low`, `medium`, `high`, `xhigh`)                     |
| `access_mode`      | Default runtime access mode (`default` or `full`)                               |

Other frontmatter keys are not part of the supported v1 contract. Avoid relying on them.

Minimal example:

```md
---
title: Reviewer
description: Reviews implementation plans and diffs before code lands.
ide: codex
reasoning_effort: high
access_mode: default
---

Review the user's request, inspect the relevant diff or files, identify concrete risks first, and
then propose the smallest safe next step. Keep the answer concise and actionable.
```

Committed fixture:

- [`docs/examples/agents/reviewer/AGENT.md`](examples/agents/reviewer/AGENT.md)

## `mcp.json`

`mcp.json` is optional and uses the standard MCP config shape with a top-level `mcpServers` object.

Example:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "${PROJECT_ROOT}"]
    },
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

Committed fixtures:

- [`docs/examples/agents/repo-copilot/AGENT.md`](examples/agents/repo-copilot/AGENT.md)
- [`docs/examples/agents/repo-copilot/mcp.json`](examples/agents/repo-copilot/mcp.json)

Validation and merge rules:

- `${VAR}` placeholders expand in `command`, `args`, and `env` values when Compozy loads the agent
- a missing environment variable is a validation error; Compozy fails closed before starting the ACP session
- relative `command` paths are resolved against the agent directory
- `mcp.json` cannot declare a server named `compozy`
- agent-local MCP servers are merged after the reserved host-owned `compozy` MCP server

The reserved `compozy` MCP server is not configured in `mcp.json`. Compozy injects it automatically into ACP sessions it creates so runtimes can call the host-owned `run_agent` tool. This is the boundary to keep straight:

- `mcp.json` is for external, agent-local MCP servers that belong to one agent definition
- the reserved `compozy` server is a host capability owned by Compozy itself

Nested execution follows the same boundary:

- a child agent gets the reserved `compozy` server plus the child's own `mcp.json`
- a child agent does not inherit the parent agent's local MCP servers implicitly

That automatic host injection is what lets normal bundled skills such as `cy-idea-factory` run council advisors through `run_agent` even when the top-level session was not started with `compozy exec --agent ...`.

## Commands

List the currently resolved agents:

```bash
compozy agents list
```

Inspect one definition:

```bash
compozy agents inspect reviewer
```

Shortened example output with path lines omitted:

```text
Agent: reviewer
Status: valid
Source: workspace
Title: Reviewer
Description: Reviews implementation plans and diffs before code lands.
Runtime defaults: ide=codex model=gpt-5.4 reasoning=high access=default
MCP servers: none
Validation: OK
```

Execute an agent through the normal `exec` pipeline:

```bash
compozy exec --agent reviewer "Review the staged changes"
```

You can still combine `--agent` with normal exec controls such as `--model`, `--reasoning-effort`, `--format`, `--persist`, and `--run-id`. Explicit CLI flags win over `AGENT.md` defaults. When an inspected agent is invalid, `compozy agents inspect <name>` prints the validation report and exits non-zero so you can fix the definition before running it.
