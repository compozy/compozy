# Tools And Skills

## Contents

- Tool-first operating model
- Discovery loop
- Operator profile scope
- Tool presentation metadata
- Oversized tool results
- Marketplace discovery
- Skill loading
- Skill sources and exposure
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
denied tools; page `compozy__tool_list` with `offset`/`next_offset` for compact summaries of the
currently callable set, then
`compozy__tool_info` for one complete schema and backend descriptor.

In a managed session, resolve canonical `compozy__skill_search`/`compozy__skill_view`, then call returned references. Do not invoke the operator CLI or read skill files directly. If policy denies the native tool, report the skill as unavailable. Operators can use `compozy skill view` from their own shell.

`compozy skill` commands reject accidental use while `COMPOZY_SESSION_ID` or `COMPOZY_AGENT` is present and point to native skill tools. Treat this as a supported-path guard, not authorization: clearing environment markers or opening the same-user operator socket is outside this guard.

Hosted MCP includes structured results as JSON text as well as `structuredContent`, so clients that
consume text can read the complete bounded result. A preview may precede that JSON.

## Operator Profile Scope

For operator tool and toolset commands, use `--profile <name>` with `--workspace <id>` to select
both boundaries. HTTP/UDS tool and toolset routes accept the `profile` query selector; they require
one profile and reject `all_profiles=true`. An approval token belongs to its minting profile as well
as its tool, session, workspace, agent, and input. Keep that scope unchanged when invoking it.

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

A task-run `result_ref` is a different resource. Resolve `compozy__task_run_result` and page it by
run ID; never pass a task-run reference to `compozy__tool_artifact_read`.

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

Marketplace in the app is one extension catalog at `/marketplace`; its installed shelf opens
`/marketplace/installed`. Extension contents describe the included MCP servers, skills, tools, and
other resources. Do not reconstruct kind-specific app paths.

For structured catalog discovery, use `GET /api/marketplace` over HTTP or UDS. Read `items`, `total`,
`sources`, `revision`, `stale`, and optional `next_cursor`. Continue with the same query, profile, and
workspace. On `marketplace_cursor_stale` with `restart: true`, discard prior pages and restart.
Failed refreshes preserve cached entries; read source diagnostics before interpreting an empty list.
HTTP(S) and file catalog roots both use `v3/extensions.json` and `v3/marketplaces.json`.
Missing v3 is a source failure; there is no root-feed or v2 fallback.

Inspect `GET /api/marketplace/entries/{entry_id}?source=<source>` and optionally
`installed_name=<local-name>`. Identity is `(source_ref, entry_id)`, never the display name.
Use the returned `install_slug` with `compozy extension install`; use the installed extension's
local name for management, and follow `manage_path` rather than inventing a URL. The complete
installed inventory comes from `GET /api/extensions`, independent of catalog pagination.
`POST /api/extensions/update` with `{"all":true}` returns per-extension outcomes: a failed item does
not undo earlier successful updates. Inspect every status, even on HTTP 200.

Native `compozy__marketplace_search` reads the canonical catalog with `query`, `limit`, and
`cursor`; obsolete `kind` input fails validation. CLI discovery uses `compozy marketplace search
[query] [--cursor <opaque>] -o json`, `compozy marketplace info <entry_id> [--source <name>]`,
and `compozy marketplace refresh`. Use the returned revision and continuation cursor when paging.
Installed-skill metadata stays local through `compozy skill info <name>`.

Entries carry the daemon's pre-install `trust` report. Read `decision`, `registry_tier`,
`allow_unverified`, and `warnings`; `checksum_verified` remains false until download verification.
Curated detail carries an HTTPS `artifact_url` and `digest_sha256`. The daemon installs that exact
archive and rejects digest mismatches regardless of unverified-install consent. GitHub, Git, and
local-folder installs have their own policy and consent gate.

Marketplace installs extension packages through the extension install API/CLI. Inspect the
entry's digest-pinned manifest inputs before installing. Secret input refs must belong to the
same extension instance; manual MCP credentials are not imported into extension inputs.
On `extension_inputs_required`, collect the missing fields described by `input_definitions`
and retry the same scoped operation with typed inputs. Definitions come from the validated
candidate and contain no stored values or secret refs. A native batch failure keeps this
metadata in `operation_error` alongside completed updates; retry the failed target only.

Manual MCP definitions remain managed through `GET /api/settings/mcp-servers` and
`PUT /api/settings/mcp-servers/{name}` with their exact scope. Existing MCP sidecars and credentials
remain in place. For extension-owned server reads, auth or override changes, pass
`owner=extension:<installed-name>` (CLI `--owner`); omitting owner addresses a manual definition.
Reads expose configured names and secret presence, never secret values or refs.

For a server that requires OAuth, run `compozy mcp auth login <name>` to start the daemon-owned PKCE flow.
Use `--manual` to paste the complete redirect URL, especially for a remote operator or non-loopback
HTTP bind. Use `--scope user` for the user layer, `--scope profile --profile <name>` for a personal
profile layer, `--scope workspace --workspace <id>` for a workspace layer, and combine
`--scope profile --profile <name> --workspace <id>` for a workspace-profile layer. Treat authorization as complete only when redacted status is
`authenticated` with `token_present=true`. `--timeout` bounds the whole attempt, including manual
input and exchange, and the active PKCE session expiry may shorten it.

For remote MCPs configured with `method: oauth` and `registration: auto`, the daemon resolves
protected-resource metadata, then the client metadata document, then makes one Dynamic Client
Registration fallback attempt.

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

Extension source search is separate: `compozy extension search <query> [--sources curated,github]
[--limit N] [--cursor <opaque>]` and native `compozy__extensions_search` page
`GET /api/extensions/search`, tagging rows with `source`, `tier`, `integrity`, and `digest_matched`
and naming any failed or slow source in `sources_degraded`. Use `compozy skill info
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

## Skill Sources And Exposure

Scan roots beyond CompozyOS's own come from `skills.sources` (folder conventions) and
`skills.custom_sources` (exact directories), resolved live through four config overlays.
`compozy__config_set` and `compozy__config_unset` write both keys at user and workspace scope and
refuse agent and profile scope with `config_scope_not_allowed`. Before reasoning about which roots
are active or proposing a source change, read the Skill sources section of `references/configuration.md`.

Inspect with `compozy skill sources -o json` or `GET /api/settings/skills`. Per root read `exists`,
`readable`, `scanned_count` (candidates found), `skill_count` (winners contributed), `truncated`,
`skipped_links`, `collisions`, and per-root `native_readers`. A root with `readable: false` omits its
counts, and an omitted count means unknown — report it as unknown. Skill catalogs, sources, and
settings answer for exactly one profile — the session's own — and no all-profiles skill view exists.

Expose links a user-owned or workspace-owned skill into an enabled preset root. Operators use
`compozy skill expose|unexpose <name> --to <targets>` and `compozy skill create <name> --expose
<targets>`; there is no native expose tool, so a managed session uses `POST
/api/skills/{name}/expose|unexpose` over HTTP or UDS under the same authorization gate as skill
enable/disable. Profile- and workspace-profile-owned skills refuse with `profile_skill_not_exposable`
before any filesystem write, and custom sources are never valid targets. Every failure is one
`expose_failed` envelope carrying per-target `results[]`; a multi-target expose rolls completed
targets back. CompozyOS never copies skill content and never removes a link it does not own.

Skills whose winning copy already lives in a root the session's own provider reads natively are
omitted from that session's injected catalog only — `claude` reads the `claude` origin, `openclaw`
and `hermes` read `agents`. They remain in the command catalog, in every list, and in explicit reads.

The bundled runtime skill named `compozy` is reserved. Same-named external definitions may keep a
source-qualified command such as `/agents:compozy`, but that command resolves the daemon's current
bundled body and resources. External copies never replace this control-plane contract.

## Session Command Catalog

Read the command catalog for the exact session before referring an operator to a slash command:

    compozy session commands <session-id> -o json

The HTTP/UDS equivalent is `GET /api/workspaces/{workspace_id}/sessions/{session_id}/commands`.
Inside a bound session, resolve canonical `compozy__command_list` and pass `session_id`. The catalog
contains daemon controls, ACP-advertised commands, and only the enabled, active skills effective for
that session. `/run` is reserved and absent.

Daemon and ACP controls are standalone prompts. Skill tokens can appear after any whitespace boundary,
including in the middle of a prompt; repeated references to one exact skill activate it once. Bare
tokens name the effective bundled, marketplace, user, profile, additional, workspace,
workspace-profile, or agent-local winner. The active profile and workspace-profile layers follow the
selection rules in `references/profiles.md`. Extension skills use `/extension-id:skill`; Marketplace
skills use `/registry-id:skill`; a skill shadowed by a same-named winner stays invocable as
`/origin:skill`, with a deterministic `origin-<hash>` prefix when two roots share one origin.

Slash activation belongs to authenticated operator prompt ingress. Agent-authored prompts and
`compozy__session_prompt` keep slash-shaped text literal. When a catalog row identifies a skill, pass
its opaque `id` as `command_id` to the session-bound `compozy__skill_view`; this reads the exact source
without exposing or reconstructing a filesystem path. If that source is no longer effective, treat
the unavailable result as terminal and read the catalog again.

## Bundled Skill Resources

The bundled `compozy` skill ships `SKILL.md` plus flat `references/*.md` resources.

Bundled `spec-cycle` globally publishes exactly `cy-create-spec`, `cy-create-tasks`, `cy-execute-task`, `cy-orchestrate-tasks`, `cy-workflow-memory`, `cy-review-round`, `cy-fix-reviews`, `cy-final-verify`, and `git-rebase` to managed sessions. Operators inspect them with `compozy skill list|view`; managed sessions use the native skill tools. Workspace definitions shadow only locally.

## Skill Provenance And Shadows

Every skill list/detail payload includes resolver provenance. `provenance.precedence_tier` names the winning tier — `bundled`, `marketplace`, `user`, `profile`, `additional`, `workspace`, `workspace_profile`, or `agent_local` — and installed-from metadata identifies extension ownership when present. The separate `origin` field names the source root's convention or custom slug and is empty for CompozyOS-native skills; `compozy__skill_list`, `compozy__skill_search`, and `compozy__skill_view` all carry it, and `compozy__skill_view` adds reconciled `exposures[]{target, path, status}` with status `healthy`, `missing`, `broken`, or `foreign_conflict`. Tier and origin are independent: never infer one from the other.

Except for the reserved `compozy` runtime skill, multiple declarations use normal precedence and
record losing declarations as shadows. Use these surfaces before assuming which skill body is active:

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
Terminal discovery starts from its complete toolset entry there; operating rules stay in references/terminal.md.

## Management-Surface Exceptions

Keep these on operator CLI, HTTP, or UDS surfaces unless CompozyOS explicitly exposes a scoped tool:

- daemon lifecycle, sockets, host/port, sandbox, provider bootstrap, and destructive repair
- creating, stopping, or mutating arbitrary sessions outside scoped authority
- MCP OAuth login/logout and browser-based auth
- trust roots, raw secrets, OAuth credentials, provider API-key bindings, PKCE material, and MCP auth secrets
- cross-session terminal-state mutation

Read-only inspection tools may exist for these domains. Do not invent a mutating tool call.
