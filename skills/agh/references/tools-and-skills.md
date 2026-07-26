# Tools And Skills

## Contents

- Tool-first operating model
- Discovery loop
- Tool presentation metadata
- Oversized tool results
- Marketplace discovery
- Skill loading
- Bundled skill resources
- Skill provenance and shadows
- Native AGH tool map
- Management-surface exceptions
- Skill authoring rules
- Reference-system lessons

## Tool-First Operating Model

AGH exposes runtime capabilities through a policy-filtered tool registry. Prefer native AGH tools over equivalent agh shell commands when a dedicated tool is callable. Tool calls are structured, policy-aware, observable, and easier to redact and audit.

Use shell commands for repository work, explicit operator requests, and management flows AGH keeps outside the normal tool-call loop.

## Discovery Loop

Use this sequence for AGH-native work:

1. Resolve canonical `agh__tool_search`, then search using the runtime domain or action.
2. Resolve canonical `agh__tool_info`, then inspect the selected ToolID before first invocation.
3. Invoke the returned dedicated tool reference with the descriptor's input schema.
4. Diagnose denied or missing tools from reason codes before changing surface.

`agh__*` names are canonical IDs, not harness call names. Use them for registry, policy, CLI, descriptors, and `tool_id`; call only the reference the harness returns.

Hosted MCP projects the full availability-gated callable catalog for a bare managed session. AGH no
longer caps that projection to bootstrap/catalog tools unless the agent definition or session
lineage explicitly narrows it. Use `agh__tool_search` and `agh__tool_info` to diagnose known but
denied tools; use `agh__tool_list` when you need only the currently callable set.

For skills, resolve canonical `agh__skill_search`/`agh__skill_view`, then call returned references. Use CLI fallback only when denied, absent, or explicitly requested.

## Oversized Tool Results

A truncated tool result can carry a bounded `preview` and an opaque
`agh://tool-artifacts/art_<sha256>` reference. Keep using the preview for immediate context, then
resolve canonical `agh__tool_artifact_read` and page the exact retained result with the returned
tool reference. Pass the artifact URI unchanged; continue from `next_offset` until `eof`.

The artifact is readable only from its owning workspace. Missing, expired, and foreign-workspace
references share the same not-found result, so do not infer whether another workspace owns one.
Operator fallback is `agh tool artifact read <artifact-uri> --workspace <workspace> [--offset N]
[-o json]`; human output writes the exact page bytes, while structured output carries base64 bytes
and paging metadata. A `result_persistence_failed` tool error preserves a bounded partial result but
does not promise a durable artifact; inspect the partial result and do not fabricate or retry a URI.

## Tool Presentation Metadata

Descriptor presentation is optional and workspace-scoped. Extension manifests use
`friendly_verb` and `preview` under `[resources.tools.<name>]`; MCP tool `_meta` uses
`agh/friendly_verb` and `agh/preview`. AGH resolves the active descriptor through the current
workspace's registry projection.

`friendly_verb` is one line and at most 80 runes. `preview` accepts only `auto`, `none`, `command`,
`path`, `delegate`, `query`, or `arg:<field>`; an argument strategy must select a non-sensitive
scalar field. The daemon selects and redacts the preview. See [Tool progress in
bridges](https://agh.network/runtime/core/bridges/progress) for the rendering and validation
contract.

## Marketplace Discovery

Use `agh__marketplace_search` for read-only MCP, extension, skill, and bundle discovery. Results carry
stable `entry_id` values and scoped installed state. CLI fallback:
`agh marketplace search [query] [--kind mcp|extension|skill|bundle] [--scope global|workspace]
[--workspace <id>] [--cursor <opaque>] -o json`. Continuation requires one kind and unchanged query,
scope, and workspace. Curated/bundle cursors fence the source; remote-skill cursors validate the prior
page boundary; grouped search omits cursors. Restart from page one after rejection. Human/TOON output
adds a Page block; JSONL adds a `type: "page"` record after items.

Exact detail is `agh marketplace info <kind> <entry_id> [--installed-name <name>]`; installed identity
applies to MCPs, extensions, and skills, never bundles. Global is default; workspace requires an ID.
Refresh with `agh marketplace refresh [--kind]` or `POST /api/marketplace/refresh`; bundles are derived.
Read each kind's `stale`, `error_class`, and `error`: failed refreshes preserve the last good rows.
Installed HTTP/UDS and structured CLI rows use `installed_name` for lifecycle mutations; `name` is
feed-owned and `manage_path` is an opaque presentation path to follow, not reconstruct.

Extension rows carry the daemon's pre-install `trust` report. Use its `decision`, `registry_tier`,
`allow_unverified`, and `warnings` directly; `checksum_verified` remains false until download verification.
Curated extension detail also carries an absolute HTTPS `artifact_url`. The daemon downloads that
exact feed-owned archive and verifies `digest_sha256` before extraction; it does not guess among
GitHub release assets. Manual non-curated installs continue through the configured registry.
For non-curated side-loads, `extension_unverified_policy_blocked` means the live
`extensions.marketplace.allow_unverified` gate is false; its diagnostic points to
`/settings/extensions` and the config key. `extension_archive_digest_mismatch` means curated bytes
do not match the catalog pin; do not retry with `--allow-unverified`.

Install MCP catalog entries with
`agh mcp install <entry> --scope global|workspace [--workspace <id>] -o json` or
`POST /api/settings/mcp-servers/install`; no mutating native install tool exists. `--set KEY` reads
one field from stdin/hidden prompt; `--vault-ref KEY=vault:mcp/...` binds a present ref. Confidential
OAuth accepts exactly one of `--oauth-client-secret` or `--oauth-client-secret-vault-ref`.

Reads expose configured field names/OAuth-secret presence, never refs. JSON returns provenance,
full config `apply` truth, and `next_step=authorize` only for OAuth. Failed apply means desired config
needs its returned repair action, not that runtime is active. HTTP/UDS requires `values` (`null` when
input-free). `mcp_install_event_persist_failed` warns that install committed but its Marketplace
event did not. Cleanup touches only superseded owned refs. Complete secret restoration rolls back;
partial secret/definition restoration retains the commit and returns a residual-state warning.

When `next_step=authorize`, run `agh mcp authorize <name>`; `agh mcp auth login <name>` reaches the
same daemon-owned PKCE flow. Use `--manual` to paste a code or full redirect URL, especially for a
remote operator or non-loopback HTTP bind. Workspace targets always carry both
`--scope workspace --workspace <id>`. Treat authorization as complete only when redacted status is
`authenticated` with `token_present=true`. `--timeout` bounds the whole attempt, including manual
input and exchange, and the active PKCE session expiry may shorten it.

Catalog OAuth templates are born-valid: they require `client_id` plus either an absolute HTTP(S)
`issuer_url` or the complete absolute `authorization_url`/`token_url` pair. Treat validation failure
as a feed-authoring error; the last valid stale projection remains authoritative.

Authorization is bound to the exact scoped server definition. Replacing or deleting that definition
invalidates pending completion, and a stored token is never sent when the transport, remote URL, or
OAuth settings no longer match. A mismatched or pre-fingerprint token remains stored until explicit
logout but status reports that login is required; begin a new authorization for the current definition.

HTTP/UDS auth routes `/auth/begin`, `/auth/exchange`, and `/auth/logout` use explicit `scope` and
optional `workspace_id`; begin requires `mode: "automatic" | "manual"`, with manual creating a fresh
paste session. The HTTP-only callback auto-completes only on loopback, follows the effective listener
(including IPv6), and returns documented `503` HTML when unavailable. Keep exchange codes/redirects
out of status and events.

Inspect MCP management truth with `agh__mcp_status`, `agh__mcp_auth_status`, or
`GET /api/settings/mcp-servers`. Configuration, authorization, runtime, and probe are independent
signals; `configured` alone never means ready. Edit stdio or remote HTTP/SSE definitions through
`PUT /api/settings/mcp-servers/{name}` with explicit scope. A generic edit clears provenance; OAuth
repair requires `authenticated` plus `token_present=true`. Reads project `env_keys` and
`secret_env_keys`, never values/refs. Preserve exact-target fields with `preserve_env` or
`preserve_secrets`; renames and target changes require replacement.

The singular `agh skill search`, `agh skill info <entry_id>`, and `agh extension search` commands
read the same discovery namespace. Use `agh skill inspect <installed-name>` for effective metadata
and resources. Do not call the deleted skill- or extension-specific browse endpoints or invent a
per-extension native search tool.

## Skill Loading

Catalogs carry names and descriptions only. Resolve canonical skill search/view through the active
harness; CLI uses `agh skill view agh` or
`agh skill view agh --file references/network.md` for a resource.

Repeated `<current-available-skills>` or `<agh-situation-context>` sections may be `unchanged`.
Reuse the latest full section for that ACP session and workspace; live surfaces remain authoritative.

`metadata.agh.when` offers a skill only when every gate family passes. `platforms` and
`environments` match any value; `requires_tools` and `requires_capabilities` require all values.
Platform means canonical Go OS, tools come from the callable session projection, and capabilities
come from the effective authored agent. Environment gates fail closed because the daemon
provides no environment context.

Inactive skills stay manageable and readable with structured reasons but are absent from catalogs.
Keep administrative `enabled` separate from runtime `activation.active`.
Tool-gated skills re-evaluate on the next projection without a daemon restart.

## Bundled Skill Resources

Bundled AGH skills are compiled from the repository skills/<name>/ directories. The canonical AGH bundled skill is agh. It includes SKILL.md and flat references/\*.md resource files.

Resource files are load-bearing. A summary in SKILL.md is never a substitute for reading the referenced file selected by the router.

## Skill Provenance And Shadows

Every skill list/detail payload includes resolver provenance. `provenance.precedence_tier` names the winning tier, and installed-from metadata identifies bundle or extension ownership when present.

When multiple declarations use the same skill name, AGH keeps the normal precedence order and records losing declarations as shadows. Use these surfaces before assuming which skill body is active:

    agh skill where <name> --workspace <ref> --for-agent <agent>
    GET /api/skills/{name}/shadows?workspace=<ref>&for_agent=<agent>

The response shape is `SkillShadowsRecord` / `SkillShadowsResponse`: `winner` is the effective declaration, and each entry in `shadows` carries `path`, `tier`, `resolved_to_winner`, and `detected_at`. The winning entry is marked `resolved_to_winner: true`; lower-precedence declarations remain visible with `false`.

Do not diagnose skill drift from filesystem paths alone. Use the resolver view so workspace, agent-local, bundled, marketplace, extension, and additional-path precedence are all considered.

Marketplace install can write files and still fail discovery verification when the effective skill is
disabled, shadowed by a higher-precedence declaration, missing marketplace provenance, or reporting a
different slug. Treat a marketplace unavailable or not-discoverable install result as terminal until
local state changes. Use `agh skill where <name>`, inspect the winning source and path, then enable
the installed skill, remove or rename the shadowing declaration, or remove the broken install
directory before retrying.

## Native AGH Tool Map

Inside AGH, read references/native-tools.md before choosing a tool or CLI fallback. It lists daemon-native toolsets and stable `agh__*` IDs, but parameters and availability come from the live descriptor returned by canonical `agh__tool_info`.

## Management-Surface Exceptions

Keep these on operator CLI, HTTP, or UDS surfaces unless AGH explicitly exposes a scoped tool:

- daemon lifecycle, sockets, host/port, sandbox, provider bootstrap, and destructive repair
- creating, stopping, or mutating arbitrary sessions outside scoped authority
- MCP OAuth login/logout and browser-based auth
- trust roots, raw secrets, OAuth credentials, provider API-key bindings, PKCE material, and MCP auth secrets
- cross-session terminal-state mutation

Read-only inspection tools may exist for these domains. Do not invent a mutating tool call.

## Skill Authoring Rules

AGH skills follow progressive disclosure:

- Keep SKILL.md short and under the practical 500-line ceiling.
- Put heavy contracts in flat one-level references/\*.md files.
- Put the Required Reading Router near the top.
- Use hard STOP directives before steps that require reference content.
- Do not nest reference-to-reference dependencies.
- Add ## Contents to every reference file that might be partially read.

For this agh skill, do not add scripts. It is a documentation and routing bundle.

## Reference-System Lessons

Hermes distinguishes skills from tools: use skills for procedural guidance and shell workflows; use tools for authenticated, precise, binary, streaming, or realtime work. OpenClaw keeps skill precedence separate from tool allowlists and injects compact prompt catalogs with local paths. Claude Code loads directory-format skill-name/SKILL.md, tracks skill roots for resources, and supports hooks from skill metadata.

AGH follows the same lesson: one compact catalog entry, explicit resource loading, daemon-owned authority, and structured tool surfaces for state changes.
