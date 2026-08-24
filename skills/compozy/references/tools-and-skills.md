# Tools And Skills

## Contents

- Tool-first operating model
- Discovery loop
- Tool presentation metadata
- Oversized tool results
- Marketplace discovery
- Skill loading
- Session command catalog
- Bundled skill resources
- Skill provenance and shadows
- Native CompozyOS tool map
- Management-surface exceptions

## Tool-First Operating Model

CompozyOS exposes runtime capabilities through a policy-filtered tool registry. Prefer native CompozyOS tools over equivalent compozy shell commands when a dedicated tool is callable. Tool calls are structured, policy-aware, observable, and easier to redact and audit.

Use shell commands for repository work, explicit operator requests, and management flows CompozyOS keeps outside the normal tool-call loop.

## Discovery Loop

Use this sequence for CompozyOS-native work:

1. Resolve canonical `compozy__tool_search`, then search using the runtime domain or action.
2. Resolve canonical `compozy__tool_info`, then inspect the selected ToolID before first invocation.
3. Invoke the returned dedicated tool reference with the descriptor's input schema.
4. Diagnose denied or missing tools from reason codes before changing surface.

`compozy__*` names are canonical IDs, not harness call names. Use them for registry, policy, CLI, descriptors, and `tool_id`; call only the reference the harness returns.

Hosted MCP projects the full availability-gated callable catalog for a bare managed session. CompozyOS no
longer caps that projection to bootstrap/catalog tools unless the agent definition or session
lineage explicitly narrows it. Use `compozy__tool_search` and `compozy__tool_info` to diagnose known but
denied tools; use `compozy__tool_list` when you need only the currently callable set.

In a managed session, resolve canonical `compozy__skill_search`/`compozy__skill_view`, then call returned references. Do not invoke the operator CLI or read skill files directly. If policy denies the native tool, report the skill as unavailable. Operators can use `compozy skill view` from their own shell.

`compozy skill` commands reject accidental use while `COMPOZY_SESSION_ID` or `COMPOZY_AGENT` is present and point to native skill tools. Treat this as a supported-path guard, not authorization: clearing environment markers or opening the same-user operator socket is outside this guard.

## Oversized Tool Results

A truncated tool result can carry a bounded `preview` and an opaque
`compozy://tool-artifacts/art_<sha256>` reference. Keep using the preview for immediate context, then
resolve canonical `compozy__tool_artifact_read` and page the exact retained result with the returned
tool reference. Pass the artifact URI unchanged; continue from `next_offset` until `eof`.

The artifact is readable only from its owning workspace. Missing, expired, and foreign-workspace
references share the same not-found result, so do not infer whether another workspace owns one.
Operator fallback is `compozy tool artifact read <artifact-uri> --workspace <workspace> [--offset N]
[-o json]`; human output writes the exact page bytes, while structured output carries base64 bytes
and paging metadata. A `result_persistence_failed` tool error preserves a bounded partial result but
does not promise a durable artifact; inspect the partial result and do not fabricate or retry a URI.

## Tool Presentation Metadata

Descriptor presentation is optional and workspace-scoped. Extension manifests use
`friendly_verb` and `preview` under `[resources.tools.<name>]`; MCP tool `_meta` uses
`compozy/friendly_verb` and `compozy/preview`. CompozyOS resolves the active descriptor through the current
workspace's registry projection.

`friendly_verb` is one line and at most 80 runes. `preview` accepts only `auto`, `none`, `command`,
`path`, `delegate`, `query`, or `arg:<field>`; an argument strategy must select a non-sensitive
scalar field. The daemon selects and redacts the preview. See [Tool progress in
bridges](https://compozy.com/docs/bridges/progress) for the rendering and validation
contract.

## Marketplace Discovery

Use `compozy__marketplace_search` for read-only MCP, extension, and skill discovery. Results carry
stable `entry_id` values and scoped installed state. CLI fallback:
`compozy marketplace search [query] [--kind mcp|extension|skill] [--scope user|profile|workspace]
[--workspace <id>] [--profile <name>] [--cursor <opaque>] -o json`. Continuation requires one kind
and unchanged query, scope, profile, and workspace. Curated cursors fence the source; remote-skill cursors validate the prior
page boundary; grouped search omits cursors. Restart from page one after rejection. Human/TOON output
adds a Page block; JSONL adds a `type: "page"` record after items.

Exact detail is `compozy marketplace info <kind> <entry_id> [--installed-name <name>]`; installed identity
applies to MCPs, extensions, and skills. User is default; profile uses the active profile and workspace
requires an ID.
Refresh with `compozy marketplace refresh [--kind]` or `POST /api/marketplace/refresh`.
Read each kind's `stale`, `error_class`, and `error`: failed refreshes preserve the last good rows.
Installed HTTP/UDS and structured CLI rows use `installed_name` for lifecycle mutations; `name` is
feed-owned and `manage_path` is an opaque presentation path to follow, not reconstruct.

Extension rows carry the daemon's pre-install `trust` report. Use its `decision`, `registry_tier`,
`allow_unverified`, and `warnings` directly; `checksum_verified` remains false until download verification.
Curated extension detail also carries an absolute HTTPS `artifact_url` and its `digest_sha256`; the
daemon installs that exact feed-owned archive instead of guessing among GitHub release assets.
`github`, `git`, and `local_path` installs bypass this feed and carry their own install-time consent
gate, not a marketplace row.

Install MCP catalog entries with
`compozy mcp install <entry> --scope user|profile|workspace [--workspace <id>] -o json` or
`POST /api/settings/mcp-servers/install`; no mutating native install tool exists. Catalog inputs are
typed and entry-owned. Use `--set id=value` for a non-secret value, `--secret id` for a secret
entered through stdin or a hidden prompt, or `--vault-ref id=vault:mcp/...` to bind a present ref.
The catalog does not accept arbitrary environment variables, command overrides, headers, or OAuth
client-secret flags.

Reads expose configured field names/OAuth-secret presence, never refs. JSON returns provenance,
full config `apply` truth, and `next_step=authorize` only for OAuth. Failed apply means desired config
needs its returned repair action, not that runtime is active. HTTP/UDS requires `values` (`null` when
input-free). `mcp_install_event_persist_failed` warns that install committed but its Marketplace
event did not. Cleanup touches only superseded owned refs. Complete secret restoration rolls back;
partial secret/definition restoration retains the commit and returns a residual-state warning.

When `next_step=authorize`, run `compozy mcp auth login <name>` to start the daemon-owned PKCE flow.
Use `--manual` to paste the complete redirect URL, especially for a remote operator or non-loopback
HTTP bind. Use `--scope user` for the user layer, `--scope profile --profile <name>` for a personal
profile layer, `--scope workspace --workspace <id>` for a workspace layer, and combine
`--scope profile --profile <name> --workspace <id>` for a workspace-profile layer. Treat authorization as complete only when redacted status is
`authenticated` with `token_present=true`. `--timeout` bounds the whole attempt, including manual
input and exchange, and the active PKCE session expiry may shorten it.

Catalog remote OAuth entries declare `method: oauth` and `registration: auto`. The daemon resolves
protected-resource metadata, then the client metadata document, then makes one Dynamic Client
Registration fallback attempt. Treat validation failure as a feed-authoring error; the last valid
stale projection remains authoritative.

Authorization is bound to the exact scoped server definition. Replacing or deleting that definition
invalidates pending completion, and a stored token is never sent when the transport, remote URL, or
OAuth settings no longer match. A mismatched or pre-fingerprint token remains stored until explicit
logout but status reports that login is required; begin a new authorization for the current definition.

HTTP/UDS auth routes include `GET /api/settings/mcp-servers/{name}/auth/status`; it reads only the
target's redacted auth state and does not start a runtime probe. `/auth/begin`, `/auth/exchange`, and
`/auth/logout` use explicit `scope` and optional `workspace_id`; begin requires
`mode: "automatic" | "manual"`, with manual creating a fresh paste session. The HTTP-only callback
auto-completes only on the loopback origin configured by `mcp.oauth.redirect_uri`; it is not derived
from the daemon listener. Manual exchange accepts only the complete redirect URL returned by the
authorization server. Keep exchange codes/redirects out of status and events.

Inspect MCP management truth with `compozy__mcp_status`, `compozy__mcp_auth_status`, or
`GET /api/settings/mcp-servers`. Configuration, authorization, runtime, and probe are independent
signals; `configured` alone never means ready. Edit stdio or remote HTTP definitions through
`PUT /api/settings/mcp-servers/{name}` with explicit scope. A generic edit clears provenance; OAuth
repair requires `authenticated` plus `token_present=true`. Reads project `env_keys` and
`secret_env_keys`, never values/refs. Preserve exact-target fields with `preserve_env` or
`preserve_secrets`; renames and target changes require replacement.

`compozy skill search` and `compozy skill info <entry_id>` read the same skill discovery namespace.
Extension source search is separate: `compozy extension search <query> [--sources curated,github]
[--limit N] [--cursor <opaque>]` and native `compozy__extensions_search` page
`GET /api/extensions/search`, tagging rows with `source`, `tier`, `integrity`, and `digest_matched`
and naming any failed or slow source in `sources_degraded`. Use `compozy skill inspect
<installed-name>` for effective installed metadata and resources, and do not call the deleted skill-
or extension-specific browse endpoints.

## Skill Loading

Scan roots allow groups at any depth (300 `SKILL.md` max); frontmatter `name` stays identity. Scaffold
with `compozy skill create <name> --group <path>`. Managed sessions resolve skill search/view through
the harness and never fall back to the operator CLI or direct file reads.

Repeated `<current-available-skills>` or `<compozy-situation-context>` sections may be `unchanged`.
Reuse the latest full section for that ACP session and workspace; live surfaces remain authoritative.

`metadata.compozy.when` offers a skill only when every gate family passes. `platforms` and
`environments` match any value; `requires_tools` and `requires_capabilities` require all values.
Platform means canonical Go OS, tools come from the callable session projection, and capabilities
come from the effective authored agent. Environment gates fail closed because the daemon
provides no environment context.

Inactive skills stay manageable and readable with structured reasons but are absent from catalogs.
Keep administrative `enabled` separate from runtime `activation.active`.
Tool-gated skills re-evaluate on the next projection without a daemon restart.

## Session Command Catalog

Read the command catalog for the exact session before referring an operator to a slash command:

    compozy session commands <session-id> -o json

The HTTP/UDS equivalent is `GET /api/workspaces/{workspace_id}/sessions/{session_id}/commands`.
Inside a bound session, resolve canonical `compozy__command_list` and pass `session_id`. The catalog
contains daemon controls, ACP-advertised commands, and only the enabled, active skills effective for
that session. `/run` is reserved and absent.

Daemon and ACP controls are standalone prompts. Skill tokens can appear after any whitespace boundary,
including in the middle of a prompt; repeated references to one exact skill activate it once. Bare
tokens name the effective bundled, user, profile, additional, workspace, workspace-profile, or
agent-local winner. The active profile and workspace-profile layers follow the selection rules in
`references/profiles.md`. Extension skills use `/extension-id:skill`; Marketplace skills use
`/registry-id:skill`.

Slash activation belongs to authenticated operator prompt ingress. Agent-authored prompts and
`compozy__session_prompt` keep slash-shaped text literal. When a catalog row identifies a skill, pass
its opaque `id` as `command_id` to the session-bound `compozy__skill_view`; this reads the exact source
without exposing or reconstructing a filesystem path. If that source is no longer effective, treat
the unavailable result as terminal and read the catalog again.

## Bundled Skill Resources

The bundled `compozy` skill ships `SKILL.md` plus flat `references/*.md` resources.

Bundled `spec-cycle` globally publishes exactly `cy-create-spec`, `cy-create-tasks`, `cy-execute-task`, `cy-orchestrate-tasks`, `cy-workflow-memory`, `cy-review-round`, `cy-fix-reviews`, `cy-final-verify`, and `git-rebase` to managed sessions. Operators inspect them with `compozy skill list|view`; managed sessions use the native skill tools. Workspace definitions shadow only locally.

## Skill Provenance And Shadows

Every skill list/detail payload includes resolver provenance. `provenance.precedence_tier` names the winning tier, and installed-from metadata identifies extension ownership when present.

When multiple declarations use the same skill name, CompozyOS keeps the normal precedence order and records losing declarations as shadows. Use these surfaces before assuming which skill body is active:

    compozy skill where <name> --workspace <ref> --for-agent <agent>
    GET /api/skills/{name}/shadows?workspace=<ref>&for_agent=<agent>

The response shape is `SkillShadowsRecord` / `SkillShadowsResponse`: `winner` is the effective declaration, and each entry in `shadows` carries `path`, `tier`, `resolved_to_winner`, and `detected_at`. The winning entry is marked `resolved_to_winner: true`; lower-precedence declarations remain visible with `false`.

Do not diagnose skill drift from filesystem paths alone. Use the resolver view so workspace, agent-local, built-in, marketplace, extension, and additional-path precedence are all considered.

Marketplace install can write files and still fail discovery verification when the effective skill is
disabled, shadowed by a higher-precedence declaration, missing marketplace provenance, or reporting a
different slug. Treat a marketplace unavailable or not-discoverable install result as terminal until
local state changes. Use `compozy skill where <name>`, inspect the winning source and path, then enable
the installed skill, remove or rename the shadowing declaration, or remove the broken install
directory before retrying.

A successful marketplace install or update may carry `cleanup_diagnostics` after its filesystem commit.
Treat each `operation` as a cleanup warning, not as rollback or permission to retry the mutation. JSON
keeps the structured array; human and TOON output list the operations. Inspect the named cleanup step
and local staging state before performing manual cleanup.

## Native CompozyOS Tool Map

Inside CompozyOS, read references/native-tools.md before choosing a tool or CLI fallback. It lists daemon-native toolsets and stable `compozy__*` IDs, but parameters and availability come from the live descriptor returned by canonical `compozy__tool_info`.

## Management-Surface Exceptions

Keep these on operator CLI, HTTP, or UDS surfaces unless CompozyOS explicitly exposes a scoped tool:

- daemon lifecycle, sockets, host/port, sandbox, provider bootstrap, and destructive repair
- creating, stopping, or mutating arbitrary sessions outside scoped authority
- MCP OAuth login/logout and browser-based auth
- trust roots, raw secrets, OAuth credentials, provider API-key bindings, PKCE material, and MCP auth secrets
- cross-session terminal-state mutation

Read-only inspection tools may exist for these domains. Do not invent a mutating tool call.
