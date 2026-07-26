# Agent Definitions

## Contents

- Files and precedence
- Minimal AGENT.md
- Fields
- Tool grants
- Providers and MCP
- Fleet reads
- Lifecycle reads and mutations
- Setup workflow
- Provider aliases and settings apply

## Files And Precedence

AGH agent definitions live in AGENT.md files with YAML frontmatter and a Markdown prompt body. User-authored global agents live under $AGH_HOME/agents/<name>/AGENT.md; workspace agents live under <workspace>/.agh/agents/<name>/AGENT.md. Extension-provided agents participate as global candidates while keeping their extension-local AGENT.md as the effective authored definition.

Runtime configuration starts from $AGH_HOME/config.toml, then workspace configuration can overlay it with <workspace>/.agh/config.toml. Agent-local skills and MCP sidecars are resolved after the effective agent definition is chosen.

Workspace definitions shadow same-name global definitions; AGH never merges them. Structured reads expose `origin`, `workspace_id`, `skills`, and `definition_digest`, so use the daemon result instead of guessing precedence from paths.

## Minimal AGENT.md

    ---
    name: general
    provider: claude
    model: claude-sonnet-5
    reasoning_effort: max
    permissions: approve-all
    ---
    You are a reliable software engineering agent.

The prompt body is required. AGH rejects an agent definition with no prompt.

## Fields

- name is required and must match the directory name for filesystem-loaded agents.
- provider, model, reasoning_effort, and command can be omitted when defaults supply them.
- reasoning_effort is `none|minimal|low|medium|high|xhigh|max`; a session override wins over AGENT.md, which wins over the selected curated model's default effort. Empty after that cascade keeps the provider/adapter default.
- tools grants exact ToolIDs or namespace-prefix wildcard patterns.
- toolsets grants named ToolsetIDs such as agh\_\_catalog.
- deny_tools narrows grants.
- permissions must be one of deny-all, approve-reads, or approve-all.
- category_path is display-only hierarchy and must be an array.
- mcp_servers declares per-agent MCP servers.

Do not use categories or slash strings for hierarchy. They are not runtime semantics.

## Tool Grants

Do not add `agh__bootstrap` or `agh__catalog` only for discovery. Leaving `tools` and
`toolsets` empty means the agent has no agent-local allowlist; hosted MCP then projects the full
availability-gated callable catalog subject to runtime policy, denylists, session lineage, and
approval gates.

Keep frontmatter grants narrow and intentional. Add `tools` or `toolsets` only when the agent should
be limited to that explicit runtime surface.

## Managed Bundled Agent

AGH ensures one managed agent definition exists on first boot and during `agh install`:

- `general` — the default public general-purpose agent (`defaults.agent`). It is the agent operators see in public agent lists and the workspace sidebar unless a workspace-local `general` overrides it.

It is recreated only when missing; operator edits are preserved.

## Reserved Background-Role Identities

`coordinator` and `dreaming-curator` are virtual AGH-owned identities, not managed or authored
agent definitions. They resolve for the coordinator, dream, and checkpoint-summary roles without an
`AGENT.md`, use embedded fixed prompts, and stay absent from public fleet/catalog reads.

Do not create, update, rename, duplicate, or bundle-materialize either name. Every authoring surface
returns `agent_name_reserved` and leaves the filesystem and catalog unchanged. `general` is not
reserved; it remains the editable managed public agent described above.

Background routing lives under `[roles.coordinator]`, `[roles.dream]`,
`[roles.checkpoint_summary]`, `[roles.memory_extractor]`, `[roles.auto_title]`, and
`[roles.memory_controller]`. A non-empty session-role `agent` selects an authored definition; it
does not customize an embedded builtin prompt. Read the effective projection with `agh roles list`
or `agh roles show <role>` before diagnosing provider or model behavior.

## Providers And MCP

Built-in provider names include claude, codex, gemini, opencode, copilot, cursor, kiro, and pi. Provider config can supply launch command, default model, API key environment, and provider-level MCP servers.

Agent `model` and `reasoning_effort` values are applied through active ACP `configOptions` before the first prompt. AGH applies model first, replaces the option snapshot from that response, then applies effort. Empty effort sends no RPC; explicit `none` does when advertised. Exact model IDs are required: unavailable models fail with `model_unavailable`; missing or unsupported reasoning fails with `reasoning_option_missing` or `reasoning_effort_unsupported`. AGH never aliases an unknown model or falls back to the provider default.

Per-agent MCP servers belong in AGENT.md or an agent-local mcp.json sidecar. mcp.json replaces same-name frontmatter servers. Use provider-level MCP when every agent for that provider needs the server; use agent-level MCP when one agent needs it.

## Fleet Reads

Use `GET /api/agents/catalog?workspace=<ref>` over HTTP or UDS when a workspace fleet needs an exact `name` filter, server-owned search, category/status filters, cursor pagination, or exact per-agent session metrics. Keep the same `name`, `q`, `category`, `status`, and `limit` when following the opaque `page.next_cursor`.

When `sessions_available` is true, each result's `sessions` object covers every visible retained session for that workspace and agent. It reports `total`, currently `active`, terminal `failed`, summed `runtime_seconds`, and `last_activity_at`. Runtime spans creation to the current read for active sessions and creation to the last persisted update for stopped sessions. Failed includes stopped sessions with a persisted failure or an `agent_crashed`/`error` stop reason. `last_activity_at` follows the session catalog's canonical activity timestamp.

`facets.total` and `facets.categories` describe the complete workspace-visible fleet before filters. `facets.active` and `facets.idle` are exact only when `sessions_available` is true. Otherwise, treat every omitted `sessions` value and metric as unavailable; do not infer zero, idle state, runtime, failure count, or last activity, and do not apply a client-side status filter.

## Lifecycle Reads And Mutations

Use structured CLI output for agent lifecycle work:

    agh agent info <name> --workspace <ref> -o json
    agh agent update <name> --workspace <ref> --expected-digest <digest> [overrides] -o json
    agh agent duplicate <source> <new-name> --workspace <ref> [--scope global|workspace] [overrides] -o json
    agh agent delete <name> --workspace <ref> --yes -o json

Create, update, and duplicate accept repeatable `--disable-skill <name>` flags. On update,
providing the flag replaces `skills.disabled`; pass `--disable-skill ""` to clear the list.

`update` replaces the complete effective authored definition, including an extension-local winner, and requires the `definition_digest` from the last read. A 409 means the digest is stale: reload, reapply the intended change, and retry with the new digest.

`duplicate` copies the whole authored directory on the daemon side, including soul, heartbeat, MCP, and other sidecars, then applies explicit AGENT.md overrides. The target must not exist. `delete` removes the effective authored directory but preserves session and event history. Deleting a workspace winner can reveal a same-name global definition; inspect `unshadowed_origin` in the response.

The matching daemon endpoints are `PUT /api/agents/:name`, `DELETE /api/agents/:name`, and `POST /api/agents/:name/duplicate`. No native update/delete/duplicate tool exists; use CLI or HTTP/UDS rather than inventing an `agh__agent_*` mutation.

## Setup Workflow

1. Set common defaults in $AGH_HOME/config.toml.
2. Create $AGH_HOME/agents/<name>/AGENT.md or workspace-local equivalent.
3. Keep frontmatter small and put behavior in the Markdown body.
4. Add only the toolsets and MCP servers the agent actually needs.
5. Reconcile desired config with runtime truth after config edits, using `agh config reload -o json` when the daemon is running.
6. Validate with AGH CLI/API rather than guessing from file shape.

If AGH rejects the agent, inspect missing name, invalid permissions, empty prompt body, malformed mcp_servers, or a directory/name mismatch first.

## Provider Aliases And Settings Apply

Provider aliases are small built-in conveniences, not user-configured compatibility keys. `claude-code` resolves to the canonical `claude` provider; aliases such as `ai-gateway`, `vercel`, `kimi`, `glm`, `x.ai`, `grok`, `open-code`, and `qwen` resolve before launch. Config files must still reference canonical provider IDs, and the removed `providers.<id>.aliases` key is rejected.

Settings writes are governed by the config apply lifecycle. Provider model-only changes are live; provider command/auth changes remain restart-required. After config edits, inspect `lifecycle`, `applied`, `next_action`, `active_generation`, and `apply_record_id` in the command response or `agh config apply-history -o json`.
