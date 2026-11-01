# Agent Definitions

## Contents

- Files and precedence
- Minimal AGENT.md
- Fields
- Unrestricted workspace access
- Tool grants
- Providers and MCP
- Fleet reads
- Lifecycle reads and mutations
- Setup workflow
- Provider aliases and settings apply

## Files And Precedence

CompozyOS agent definitions live in AGENT.md files with YAML frontmatter and a Markdown prompt body. User-authored global agents live under $COMPOZY_HOME/agents/<name>/AGENT.md; workspace agents live under <workspace>/.compozy/agents/<name>/AGENT.md. Agents from enabled published extensions participate as global candidates; agents from active development links participate only in the linked workspace. Both keep their extension-local AGENT.md as the effective authored definition.

Runtime configuration starts from $COMPOZY_HOME/config.toml, then workspace configuration can overlay it with <workspace>/.compozy/config.toml. Agent-local skills and MCP sidecars are resolved after the effective agent definition is chosen.

Workspace definitions shadow same-name global definitions; CompozyOS never merges them. Structured reads expose `origin`, `workspace_id`, `skills`, and `definition_digest`, so use the daemon result instead of guessing precedence from paths.

## Minimal AGENT.md

    ---
    name: general
    provider: claude
    model: claude-sonnet-5
    reasoning_effort: max
    permissions: approve-all
    ---
    You are a reliable software engineering agent.

The prompt body is required. CompozyOS rejects an agent definition with no prompt.

## Fields

- name is required. Outer whitespace is trimmed, and the result must match `^[a-z][a-z0-9_-]{0,105}$`: start with a lowercase ASCII letter, then use lowercase ASCII letters, numbers, hyphens, or underscores; the maximum length is 106 characters. Filesystem-loaded agents must also match the directory name.
- provider, model, reasoning_effort, speed, acp_options, and command can be omitted when defaults supply them.
- reasoning_effort is `none|minimal|low|medium|high|xhigh|max|ultra`; a session override wins over AGENT.md, which wins over the selected curated model's default effort. Empty after that cascade keeps the provider/adapter default.
- speed is `normal|fast`; prompt and durable session selection override AGENT.md. Each acp_options entry requires `id` and exactly one of `value_id` or `bool_value`.
- tools grants exact ToolIDs or namespace-prefix wildcard patterns.
- toolsets grants named ToolsetIDs such as compozy\_\_catalog.
- deny_tools narrows grants.
- permissions must be one of deny-all, approve-reads, or approve-all. It sets static tool policy and the ordinary session cross-workspace boundary; see references/native-tools.md (Workspace Boundary).
- `workspace_access` accepts only omission (default) or `unrestricted`; see Unrestricted Workspace Access below.
- category_path is display-only hierarchy and must be an array.
- mcp_servers declares per-agent MCP servers.
- session_env maps environment names to `env:NAME` or `vault:path` references, optionally expressed
  as `{ ref: "env:NAME", required: false }`. Shorthand references are required. Agent declarations
  override provider declarations by name; literals and reserved `COMPOZY_` names are rejected.
  Missing required values fail launch, while missing optional values are omitted with a diagnostic.
  This also applies to native-CLI agents and their terminals without changing provider login ownership.

Do not use categories or slash strings for hierarchy. They are not runtime semantics.

## Unrestricted Workspace Access

`workspace_access: unrestricted` lets sessions of one agent read and act on every workspace, the
User scope, and every profile, the way the operator does. It exists for stays-on agents that live in
the operator's home. It is the only grant vocabulary in CompozyOS (User Scope Work ADR-003, which
supersedes cross-workspace ADR-007 §1 for this one field); there is no grants table, config section,
or grant-management tool.

    ---
    name: companion
    workspace_access: unrestricted
    permissions: approve-reads
    ---

- **Operator-authored only.** Set or raise it through the Web agent editor, HTTP/UDS agent routes
  without agent credentials, or a file edit (there is no CLI flag). Agent-authored writes (`compozy__agent_create`, agent update
  tools, HTTP with agent headers) that set or raise it fail with `403 workspace_access_operator_only`.
  Marketplace and extension definitions cannot ship it: the loader drops the field with diagnostic
  `workspace_access_untrusted_source`. Never ask an agent to grant itself access.
- **Effective per session.** The daemon resolves `effective_workspace_access` (`""` or
  `unrestricted`) once at session start and never changes it for that session; resume keeps it.
  Operator-originated sessions get the definition's value. Every agent-originated creation (spawn,
  subagent, `compozy__session_create`, HTTP with agent credentials, tasks, automations, and loop runs
  an agent authored) gets the minimum of the authenticated creator's effective grant and the
  definition. Caller-supplied parent or provenance fields never count as the creator.
- **Deferred execution.** Tasks, automation jobs and triggers, and Loop runs persist a
  `grant_ceiling` when authored. An agent edit of an executable field (prompt, target, agent,
  schedule, inputs) narrows it; restarts and retries keep it. Records created before this release have
  no ceiling until an operator saves them again.
- **Orthogonal to tool risk.** `permissions` still governs destructive tools and approvals. The grant
  only removes the workspace-axis and profile-axis boundaries. Hooks (`tool.pre_call`) still run
  first and can deny or narrow granted calls.
- **Audit and visibility.** Every grant-authorized decision appends `workspace.access_granted` with
  source `agent_grant`, the acting session and profile, target kind (`workspace`, `user`, or
  `aggregate`), target workspace, and target profile. Session payloads expose
  `effective_workspace_access`; the Web shows an **Unrestricted** badge; `compozy agent info <name>`
  prints `workspace_access`.
- **Soft boundary.** A process inside a session can call the daemon without agent headers, and
  provider filesystem tools reach every file the OS user can. The grant is an authority statement for
  CompozyOS surfaces, not a sandbox.

Authority matrix (HTTP, UDS, and native tools enforce the same rules):

| Surface                                                                    | Operator | Ordinary agent                       | Agent with `unrestricted`            | App token         |
| -------------------------------------------------------------------------- | -------- | ------------------------------------ | ------------------------------------ | ----------------- |
| Own workspace or own User scope                                            | allow    | allow                                | allow                                | per route         |
| Another workspace, or User from a project                                  | allow    | permission mode + consent            | allow, audited                       | per route         |
| Another profile (`profile=`, `all_profiles=true`)                          | allow    | denied                               | allow, audited                       | per route         |
| All-projects aggregates                                                    | allow    | `403 agent_scope_denied`             | allow, audited (`aggregate`)         | new routes denied |
| Window, desktop, layout tools on project partitions and own-profile `user` | allow    | own workspace only                   | allow, audited                       | per route         |
| Reserved `global` partition, other profiles' partitions                    | allow    | denied                               | denied                               | denied            |
| Presence leases, notification receipts and acknowledgements                | allow    | denied                               | denied                               | per route         |
| Set or raise `workspace_access`                                            | allow    | `403 workspace_access_operator_only` | `403 workspace_access_operator_only` | denied            |
| Tool-risk approvals                                                        | n/a      | per `permissions`                    | per `permissions`                    | n/a               |

Work a granted agent creates in profile P is owned by P. Older daemons reject AGENT.md files that
contain `workspace_access` (strict decode), so upgrade every daemon that reads the file first.

## Tool Grants

Do not add `compozy__bootstrap` or `compozy__catalog` only for discovery. Leaving `tools` and
`toolsets` empty means the agent has no agent-local allowlist; hosted MCP then projects the full
availability-gated callable catalog subject to runtime policy, denylists, session lineage, and
approval gates.

Keep frontmatter grants narrow and intentional. Add `tools` or `toolsets` only when the agent should
be limited to that explicit runtime surface.

## Managed Bundled Agent

CompozyOS ensures one managed agent definition exists on first boot and during `compozy install`:

- `general` — the default public general-purpose agent (`defaults.agent`). It is the agent operators see in public agent lists and the workspace sidebar unless a workspace-local `general` overrides it.

It is recreated only when missing; operator edits are preserved.

## Reserved Background-Role Identities

`coordinator` is a virtual CompozyOS-owned identity, not a managed or authored agent definition. It
resolves for the coordinator role without an `AGENT.md`, uses an embedded fixed prompt, and stays
absent from public fleet/catalog reads.

Do not create, update, rename, duplicate, or bundle-materialize that name. Every authoring surface
returns `agent_name_reserved` and leaves the filesystem and catalog unchanged. `general` is not
reserved; it remains the editable managed public agent described above.

Background routing lives under `[roles.coordinator]` and `[roles.auto_title]`. A non-empty session-role `agent` selects an authored definition; it
does not customize an embedded builtin prompt. Read the effective projection with `compozy roles list`
or `compozy roles show <role>` before diagnosing provider or model behavior.

## Providers And MCP

Built-in provider names include claude, codex, gemini, opencode, copilot, cursor, kimi-cli, qwen-code, and pi. Provider config can supply launch command, default model, API key environment, and provider-level MCP servers. Any other ACP CLI runs as a custom `[providers.<id>]` entry with its own `command`. The retired built-ins hermes, openclaw, blackbox, cline, goose, junie, kiro, openhands, qoder, mistral, groq, and minimax still resolve until v0.5.0 with a `provider.retired_builtin` daemon-log warning whose `replacement` field holds the exact `[providers.<id>]` entry; declaring that entry silences it.

For standard ACP providers, CompozyOS applies model, replaces the option snapshot, applies Reasoning and Fast, then applies remaining ACP options in stable ID order, refreshing and revalidating after every response. Cursor resolves logical model plus option values to a private launch alias and replaces the process when that binding changes. Empty effort sends no RPC; explicit `none` does when advertised. Unknown or unsupported values fail; CompozyOS never falls back silently.

Per-agent MCP servers belong in AGENT.md or an agent-local mcp.json sidecar. mcp.json replaces same-name frontmatter servers. Use provider-level MCP when every agent for that provider needs the server; use agent-level MCP when one agent needs it.

## Fleet Reads

`compozy agent show <name> -o json` and `GET /api/agents/{name}` include effective
`session_env_status` metadata under `effective_runtime`: reference, source, presence, and whether
the declaration is required. These reads never resolve Vault plaintext. The Web Configuration
tab displays the same metadata; unrelated Settings edits preserve authored environment maps.

Use `GET /api/agents/catalog?workspace=<ref>&profile=<name>` over HTTP or UDS when a workspace fleet needs an exact `name` filter, server-owned search, category/status filters, cursor pagination, or exact per-agent session metrics. Keep the same `name`, `q`, `category`, `status`, and `limit` when following the opaque `page.next_cursor`.

Omit `workspace` for Global. Definitions use the acting profile's shared resource resolution;
session metrics aggregate visible sessions by agent name across all projects and no-workspace
work. Project-local definitions are not merged into the Global list. A workspace selector still
narrows both effective definitions and metrics. Keep the same profile while following a cursor.

When `sessions_available` is true, each result's `sessions` object covers every visible retained session for that scope and agent name. It reports `total`, currently `active`, terminal `failed`, summed `runtime_seconds`, and `last_activity_at`. Runtime spans creation to the current read for active sessions and creation to the last persisted update for stopped sessions. Failed includes stopped sessions with a persisted failure or an `agent_crashed`/`error` stop reason. `last_activity_at` follows the session catalog's canonical activity timestamp.

`facets.total` and `facets.categories` describe the complete workspace-visible fleet before filters. `facets.active` and `facets.idle` are exact only when `sessions_available` is true. Otherwise, treat every omitted `sessions` value and metric as unavailable; do not infer zero, idle state, runtime, failure count, or last activity, and do not apply a client-side status filter.

## Lifecycle Reads And Mutations

Use structured CLI output for agent lifecycle work:

    compozy agent info <name> --workspace <ref> -o json
    compozy agent update <name> --workspace <ref> --expected-digest <digest> [overrides] -o json
    compozy agent duplicate <source> <new-name> --workspace <ref> [--scope global|workspace] [overrides] -o json
    compozy agent delete <name> --workspace <ref> --yes -o json

Create, update, and duplicate accept repeatable `--disable-skill <name>` flags. On update,
providing the flag replaces `skills.disabled`; pass `--disable-skill ""` to clear the list.
They also accept `--speed`, repeatable `--acp-option id=value`, and repeatable
`--acp-toggle id=true|false` for authored runtime defaults.

Create and update accept a repeatable, ordered `--fallback-route` that replaces the agent's
`fallback_chain`, and `--clear-fallback-chain` to empty it; omitting both keeps the current chain.
Keys are `provider`, `model`, `reasoning_effort`, `speed`, and `command`; `command` must be the last
key and takes the rest of the value verbatim (quote the whole flag value in the shell):

    compozy agent update reviewer --expected-digest <digest> \
      --fallback-route 'provider=claude,model=opus-4-8,command=CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp' \
      --fallback-route provider=cursor,model=grok-4.6,reasoning_effort=high -o json

The `fallback_chain` frontmatter key holds the same route tables as `[[roles.<role>.fallback_chain]]`
(`provider` and `model` required; optional `reasoning_effort`, `speed`, `acp_options`, `command`,
`session_env`). Route environment declarations merge by name over the selected provider and agent
defaults. Use frontmatter or the typed HTTP/UDS route objects to author route `session_env`.
`compozy agent show` and `GET /api/agents/{name}` project it with `command_fingerprint` for routes that
set `command`. Invalid routes fail with a path such as
`agent "reviewer" fallback_chain[0].command: command is missing an executable`. Sessions a background
role launches with this agent run the role's chain, never the agent's.

`update` replaces the complete effective authored definition and requires the `definition_digest` from the last read. A 409 means the digest is stale: reload, reapply the intended change, and retry with the new digest.

When `--provider` selects a different underlying provider, `update` clears an omitted command, model, reasoning effort, and ACP options; equivalent built-in aliases and case variants keep those settings.

`duplicate` copies the whole authored directory on the daemon side, including soul, heartbeat, MCP, and other sidecars, then applies explicit AGENT.md overrides. The target must not exist. `delete` removes the effective authored directory but preserves session and event history. Deleting a workspace winner can reveal a same-name global definition; inspect `unshadowed_origin` in the response.

Extension AGENT.md definitions can be updated at their effective source using the same digest
check. Bundled edits survive restart while the shipped bundle is unchanged; an updated bundle can
replace them. Duplicate into an authored definition when the customization must be independent
of package updates. Package-owned agents cannot be deleted individually; disable the extension.

The matching daemon endpoints are `PUT /api/agents/:name`, `DELETE /api/agents/:name`, and `POST /api/agents/:name/duplicate`. No native update/delete/duplicate tool exists; use CLI or HTTP/UDS rather than inventing a `compozy__agent_*` mutation.

## Authored Context and Profiles

Select the intended Profile for named-agent SOUL.md and HEARTBEAT.md operations. The CLI's global
`--profile` selector is forwarded by its common transport; the corresponding HTTP read, validate,
write, delete, history, rollback, status, and wake routes accept the optional `profile` query.
Omitting it retains `default`. Request bodies and CLI subcommands are unchanged.

History and rollback belong to the winning authored source path, so same-named agents in different
Profiles cannot read or restore each other's revisions. Session targets retain their persisted
Profile; an explicit status/wake session must belong to the selected Profile and agent. Managed
extension SOUL.md and HEARTBEAT.md sources remain protected from writes.

A `SOUL.md` that still carries the retired memory-policy frontmatter key from an earlier release stays
valid: any value is accepted and ignored. `compozy agent soul validate` reports no diagnostic, the key is
never rendered into a session, and CompozyOS does not rewrite the file.

## Hidden Triage

Hidden triage requires a confirmed read-only provider mode. Claude and OpenCode
use the verified `plan` mode; Codex advertises `read-only`. Mode choices may arrive
through ACP `configOptions` without a legacy modes list. The daemon confirms a
config-option selection before prompting. Other providers' `plan` modes are not
accepted by name alone; unsupported modes fail with `triage_unsupported`.

## Stays-On Agent Memory

Stays-on agent memory is partitioned by profile, definition origin, and agent name.
Agent-authenticated memory routes and native memory tools require a daemon-resolved
instance lineage for that agent; a manually started session with the same name is
not sufficient. Operator memory routes retain their explicit target selection.
Entries record the source workspace name, or `Global`. Document and history digests
use `sha256:<hex>`; pass the complete digest back when making a digest-checked edit.

## Setup Workflow

1. Set common defaults in $COMPOZY_HOME/config.toml.
2. Create $COMPOZY_HOME/agents/<name>/AGENT.md or workspace-local equivalent.
3. Keep frontmatter small and put behavior in the Markdown body.
4. Add only the toolsets and MCP servers the agent actually needs.
5. Reconcile desired config with runtime truth after config edits, using `compozy config reload -o json` when the daemon is running.
6. Validate with CompozyOS CLI/API rather than guessing from file shape.

If CompozyOS rejects the agent, inspect the name format, invalid permissions, empty prompt body, malformed mcp_servers, or a directory/name mismatch first. Use names such as `code-reviewer` or `code_reviewer`; names with spaces or uppercase letters are invalid.

## Provider Aliases And Settings Apply

Provider aliases are small built-in conveniences, not user-configured compatibility keys. `claude-code` resolves to the canonical `claude` provider; aliases such as `ai-gateway`, `vercel`, `kimi`, `glm`, `x.ai`, `grok`, `open-code`, and `qwen` resolve before launch. Config files must still reference canonical provider IDs, and the removed `providers.<id>.aliases` key is rejected.

Settings writes are governed by the config apply lifecycle. Provider model-only changes are live; provider command/auth changes remain restart-required. After config edits, inspect `lifecycle`, `applied`, `next_action`, `active_generation`, and `apply_record_id` in the command response or `compozy config apply-history -o json`.

After native provider login, run `compozy provider auth status <provider> --remote` to probe through the daemon (`POST /api/providers/:provider_id/auth/probe` over HTTP/UDS). An `authenticated` result clears cached pre-start verdicts; retry session creation immediately. Each workspace/profile still probes its own credentials. HTTP and UDS never execute the login command.

Claude's default `claude auth status` JSON is supported directly: only a valid
`loggedIn: true` with exit zero permits authentication, while `false` requires
native login. Invalid or missing verdicts remain unknown and error classifications
retain precedence. Structured status output retains only the verdict, excluding
account and organization details from probe responses and diagnostics.

## Identity and kind

Create supports `display_name` (80 characters), `description` (280), `kind` (`on_demand` by default or `stays_on`),
and an open `appearance` object. `compozy__agent_create` accepts these fields; agent reads include `label`,
`kind`, authored `appearance`, and `resolved_appearance`. The label falls back to the humanized slug.
A generated appearance uses `{version:1, avatar:"generated", seed:<slug>, generation:1, color:"#7c5cff"}`.
Icon and emoji forms use `avatar:"icon", icon:<Lucide slug>` or `avatar:"emoji", emoji:<one emoji>` with a color.
Unsupported saved appearance falls back with a diagnostic and survives unrelated edits.

Kind is fixed at creation. Use `compozy agent duplicate <source> <name> --kind on_demand` to choose another kind.
Changing it through update returns `agent_kind_immutable`. PUT preserves omitted identity fields and ignores
`label` and `resolved_appearance`. `GET /api/agents/catalog?group=agent` groups identical slug and digest
across authorized scopes, with `items`, `scopes`, `active`, and `next_cursor`; label search uses `q`.
Extension AGENT.md files use the same keys; installing an extension never creates an instance.
