# Tools And Skills

## Contents

- Tool-first operating model
- Discovery loop
- Operator profile scope
- Tool presentation metadata
- Oversized tool results
- Marketplace discovery
- MCP activation and consent; runtime MCP inventory
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

Hosted MCP lists the availability-gated callable catalog for a bare managed session by default.
Agent definitions and session lineage can narrow the projection. If the operator enables
`tools.hosted_mcp.tool_list_threshold`, the Claude ACP bridge defers external MCP servers above the
threshold. Look for its bounded server directory in `compozy__tool_search`. Search with a server
name and an action, page `compozy__tool_list`, or inspect a known ID with `compozy__tool_info`.
Callable deferred matches are added to the current bind under their real names and use Claude's
normal schema loading. If Claude still defers a schema, load it by name with its ToolSearch.
After idle release, crash recovery, or restart, a new bind starts with its initial budget.
Rediscover deferred tools even if the transcript contains earlier calls to them.

Use the same discovery tools for denied diagnostics. Page `compozy__tool_list` with
`offset`/`next_offset` for compact summaries of the currently callable set, then read a complete
schema and backend descriptor with `compozy__tool_info`.

External MCP declarations in user, personal-profile, workspace and workspace-profile config or
`mcp.json` feed the same hosted catalog. See [MCP Activation And Consent](#mcp-activation-and-consent)
for which sources are admitted, when open sessions see a change, and how to repair a missing tool.
Providers with session MCP disabled receive no tools. Direct stdio sidecars reload on launch
configuration changes at the next message as well; remote HTTP/OAuth require hosted MCP.
Workspace/profile isolation and delegated `mcp_servers` allowlists still apply.
Server allowlists use logical declaration names; allocated runtime names remain tool/routing identities.
Ready never overrides a denial.

In a managed session, resolve canonical `compozy__skill_search`/`compozy__skill_view`, then call returned references. Do not invoke the operator CLI or read skill files directly. If policy denies the native tool, report the skill as unavailable. Operators can use `compozy skill view` from their own shell.

`compozy skill` commands reject accidental use while `COMPOZY_SESSION_ID` or `COMPOZY_AGENT` is present and point to native skill tools. Treat this as a supported-path guard, not authorization: clearing environment markers or opening the same-user operator socket is outside this guard.

Hosted MCP includes structured results as JSON text as well as `structuredContent`, so clients that
consume text can read the complete bounded result. A preview may precede that JSON.

## Operator Profile Scope

For operator tool and toolset commands, use `--profile <name>` with `--workspace <id>` to select
both boundaries. HTTP/UDS tool and toolset routes accept the `profile` query selector; they require
one profile and reject `all_profiles=true`. An approval token belongs to its minting profile as well
as its tool, session, workspace, agent, and input. Keep that scope unchanged when invoking it.

Prefer `tool approve --input <json>` with the same input you will invoke. Approval issuance binds
native workspace names, paths, and inherited operator workspace selection to the same canonical
input as dispatch. The returned `input_digest` describes that bound input. If supplying both input
and `--input-digest`, the supplied digest must match the submitted JSON before binding. Digest-only
approval requires the digest of the already-bound input. Approval digests exclude `result_view`:
compute a supplied digest on the input without the selector, or pass `--input` and let the daemon
compute it. A pre-call hook that changes the final
input still requires a matching approval; tokens remain single-use.

## Oversized Tool Results

A truncated tool result can carry a bounded `preview` and an opaque
`compozy://tool-artifacts/art_<sha256>` reference. Keep using the preview for immediate context, then
resolve canonical `compozy__tool_artifact_read` and page the exact retained result with the returned
tool reference. Pass the artifact URI unchanged; continue from `next_offset` until `eof`.

Over hosted MCP, a truncated result still returns `structuredContent`: instead of the declared payload
it carries the continuation envelope `{"truncated":true,"truncated_from_bytes":N,"preview":"...",
"artifacts":[{"uri":"compozy://tool-artifacts/..."}],"read_tool":"compozy__tool_artifact_read"}`.
Hosted native tools advertise no `outputSchema`. Structured continuations and typed
`{"error":{...}}` payloads remain unchanged. Read result contracts through `compozy__tool_info`.
Check `truncated` first and page retained results through `read_tool`. Artifact reads default to
32 KiB of decoded bytes: base64 plus the JSON text and `structuredContent` copies stay below a
128 KiB inline transport budget. Explicit reads up to 64 KiB remain supported; request smaller
pages when the harness has a lower cap.

Workspace artifacts retain workspace-only ownership. User artifacts belong to User scope of the
producing profile. Missing, expired, and foreign-scope references share the same not-found result,
so do not infer ownership.
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
scalar field. The daemon selects and redacts the preview before presenting tool activity.

MCP declarations embedded in installed Marketplace/ClawHub skills are disabled. Do not write the
retired `skills.allowed_marketplace_mcp` key. Installed skill content/provenance and manual or
extension-provided MCP servers retain their existing behavior.

## Marketplace Discovery

Source management is experimental: `compozy marketplace sources list|add|remove|refresh -o json`,
`GET/POST /api/marketplace/sources`, `PATCH/DELETE /api/marketplace/sources/{name}`, and
`POST /api/marketplace/sources/{name}/refresh` share the
same daemon-owned global configuration. Add accepts `{ref, name?}`; `?dry_run=true` inspects without
registering. PATCH takes `{enabled: boolean}`. Presets may be disabled but not removed; the CompozyOS
feed stays enabled. The read-only `compozy__marketplace_sources` tool takes `{}` and returns the
source list with state, counts, `stability: experimental`, and a `diagnostics` array.

On `marketplace_source_exists`, use `suggested_name`; on `marketplace_source_name_retained`, inspect
`retained_by` and choose another name or re-add the original ref. Reserved names cannot be reused.
`marketplace_not_a_marketplace` includes `checked` paths. Sources above 2 MiB return
`marketplace_document_too_large`; an unknown source returns `marketplace_source_not_found`.
Disabling/removing a source never removes its installed extensions. A failed refresh returns the
source's degraded state. CLI aggregate refresh fails only when all attempted sources fail.

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

Installing the same extension source in another workspace adds a scoped installation. Package
updates preserve existing installations, their inputs and profile enablement. Remove using the
selected workspace: `DELETE /api/extensions/{name}?workspace=<id>` detaches that installation and
keeps other installations; only the last removal retires the package. An active development link
in that workspace follows the development unlink lifecycle.

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
`owner=extension:<installed-name>` (CLI `--owner`) plus the logical manifest server name. Allocated
runtime names are not aliases for management, authorization or diagnostic calls. Omitting owner
addresses a manual definition. Read-only `compozy__mcp_status` and `compozy__mcp_auth_status` fall
back to the extension definition visible to the session when no manual server has that name, and
fail asking for an explicit owner when two extensions declare it.
Reads expose configured names and secret presence, never secret values or refs.

A bare HTTP remote without `auth`, `headers`, or `secret_headers` can sign in through
OAuth discovery with automatic client registration. A successful begin must discover protected-resource
metadata; a 401 alone does not make metadata or a provider grant valid. A client Agent Plugin with
declared headers also offers **Sign in** in Settings after an origin-bound
Bearer challenge advertises protected-resource metadata. Use the extension-qualified owner for
these sign-ins. Manual header-auth servers require an explicit OAuth policy to use OAuth.

For a server that requires OAuth, run `compozy mcp auth login <name>` to start the daemon-owned PKCE flow.
Use `--manual` to paste the complete redirect URL, especially for a remote operator or non-loopback
HTTP bind. Use `--scope user` for the user layer, `--scope profile --profile <name>` for a personal
profile layer, `--scope workspace --workspace <id>` for a workspace layer, and combine
`--scope profile --profile <name> --workspace <id>` for a workspace-profile layer. Treat authorization as complete only when redacted status is
`authenticated` with `token_present=true`. `--timeout` bounds the whole attempt, including manual
input and exchange, and the active PKCE session expiry may shorten it.

Configure pre-registered OAuth scopes as individual `auth.scopes` entries. Token responses can
return space-separated or comma-separated grants. After `compozy mcp check <name>` observes a 401
for the stored bearer credential, auth status reports `expired` when the credential can refresh or
`needs_login` when it requires sign-in. Run another check to trigger an available refresh. A
connection or protocol failure gives a redacted cause in the check diagnostic when available.

OAuth credentials belong to the scope that declares the effective server definition. Resolution
order, narrowest first: workspace-profile, workspace, profile, user. A workspace or profile that
only inherits a server declared at a broader scope uses that scope's credential at runtime. Status
reads for the narrower scope (`mcp auth status --scope workspace`, `compozy__mcp_status`,
`compozy__mcp_auth_status`) report that effective record with `inherited: true` (CLI shows
`user (inherited)`), and `repair_paths` name the declaring scope. Login, exchange, and logout for a
scope without its own definition fail with a validation error naming the declaring scope; authorize
at that scope, or declare the server in the narrower layer to give it its own credential.

Sign-in is availability, not consent: consent comes from where the server was declared (see
[MCP Activation And Consent](#mcp-activation-and-consent)). A project server waiting for approval
refuses `auth/begin` with `409 mcp_server_waiting_approval` before any network call. Passing its
`definition_digest` (CLI `compozy mcp auth login <name> --digest <d>`) records the approval first and
then begins sign-in; a stale digest fails `mcp_approval_definition_changed`.

For remote MCPs configured with `method: oauth` and `registration: auto`, the daemon resolves
protected-resource metadata, then the client metadata document, then makes one Dynamic Client
Registration fallback attempt.

Authorization is bound to the exact scoped server definition. Replacing or deleting that definition
invalidates pending completion, and a stored token is never sent when the transport, remote URL, or
OAuth settings no longer match. A mismatched or pre-fingerprint token remains stored until explicit
logout but status reports that login is required; begin a new authorization for the current definition.

Sign-in adds `offline_access` alongside configured or resource-advertised scopes if the issuer
advertises it and the selected client metadata or registration allows it. When neither side
specifies resource scopes, the issuer's defaults apply. The issuer controls consent and whether
it issues a refresh token. Tool discovery and execution try renewal in the final 10% of an
access token's lifetime, up to 30 seconds before expiry,
using the client bound to that target. Refresh needs no new login callback or replacement
registration.
For a client ID metadata document (CIMD), renewal uses the persisted document URL as the public
client ID and sends no client secret. CIMD registrations from earlier releases renew the same
way when their bindings still match and a refresh token is present.
If the stored registration cannot renew, including an expired or unavailable client secret,
status shows that the target needs reauthorization. Discovery makes no refresh request until the
local credentials support renewal again. A discovered replacement issuer receives no stored
credential. The old refresh grant is removed and the bound access token remains usable until expiry.
Refreshing or logging out an extension-owned MCP leaves manual MCP credentials unchanged.

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
`secret_env_keys`, never values/refs. Preserve headers and secret-header bindings for the same endpoint and write target with
`preserve_headers: true`; reads expose only `headers_configured`. Preserve other exact-target fields with `preserve_env` or
`preserve_secrets`; renames and target changes require replacement.

An `expired` OAuth status with `token_present=true` and `refreshable=true` keeps MCP tools
advertised: discovery or a call renews the token. Do not request manual sign-in solely for this
combination. Refreshes share one in-flight attempt per credential target and use jittered retry
backoff (about 1 s, doubling to at most 60 s). Transient failures retain the credentials, listed
tools, and the use of a still-valid access token. Definitive rejection (`invalid_grant`,
`revoked`, `invalid_client`, or `unauthorized_client`) clears the token and reports
`needs_login`. Expiry without a usable refresh path remains blocked. Successful sign-in or renewal
clears that server's authentication-related dead mark without clearing health-only failures.
`mcp.auth.refresh_failed` logs contain server, cause class, and attempt count, never credentials.

Extension source search is separate: `compozy extension search <query> [--sources curated,github]
[--limit N] [--cursor <opaque>]` and native `compozy__extensions_search` page
`GET /api/extensions/search`, tagging rows with `source`, `tier`, `integrity`, and `digest_matched`
and naming any failed or slow source in `sources_degraded`. Use `compozy skill info
<installed-name>` for effective installed metadata and resources, and do not call the deleted skill-
or extension-specific browse endpoints.

## MCP Activation And Consent

**Consent comes from where a server is declared.** These are admitted without another step:
servers added through CompozyOS at any scope (Settings, `compozy mcp import`, agent create), servers in
operator-owned files under `$COMPOZY_HOME` (user and personal-profile `mcp.json`/`config.toml`),
servers of an enabled extension, and copies made with Copy to CompozyOS. A server declared in a file
under a workspace root (`.compozy/mcp.json`, project `config.toml`, `.compozy/profiles/<p>/…`, or an
agent or skill sidecar under the project) and changed outside CompozyOS **waits for approval**. A
waiting server is listed as metadata only; CompozyOS never starts, connects to, discovers, probes or
signs in to it. CompozyOS writes at workspace scope record the approval for the committed definition;
if that record fails, the write returns a warning and the server stays waiting.

- Approve with `compozy mcp allow <server> --workspace <ws>` (the CLI reads the current definition and
  sends its digest), `--digest <d>` to pin the definition you inspected, or `compozy__mcp_allow`
  with `definition_digest` from `compozy__mcp_list`. The native tool always requires operator
  approval, including in `approve-all`; an agent cannot grant itself consent. Revoke with `--revoke` / `decision: "revoke"`;
  the tools stop working on the next call. The HTTP/UDS route is
  `PUT|DELETE /api/settings/mcp-servers/{name}/approval`.
- An approval binds one (profile, workspace, server name) and one definition digest. Any change to
  the definition's non-secret fields (command, args, URL, env and header keys and values, Vault
  reference identities, OAuth registration) returns it to waiting. Rotating a value behind the same
  Vault reference does not.
- `GET /api/settings/mcp-servers` items carry `approval{required,state,definition_digest,reason}`
  (`state`: `not_required`, `waiting`, `allowed`; `reason`: `new` or `changed` while waiting) and
  `callable_in_sessions`.
- `tools.policy.external_default` applies only to sources without consent: project servers waiting
  for approval. `enabled` admits them; `disabled` (default) and `ask` keep them waiting and never start
  them. `ask` still prompts per call for consented sources. Dynamic tools stay denied.
- Blocked calls name the repair: `source_disabled` "`<server>` is waiting for approval in `<ws>`.
  Allow it in Settings → MCP servers." or "`<server>` was removed from CompozyOS"; `mcp_auth_required`
  "`<server>` needs sign-in. Sign in to `<server>` in CompozyOS (Settings → MCP servers)."

**Changes apply on the next message.** Adding, enabling, approving or signing in to a server, or
installing or enabling an extension, makes its tools callable in sessions already open, from their
next message. Removing, disabling, revoking or signing out withdraws the tools on the next call. At
prompt admission CompozyOS compares the session's activation fingerprint (the advertised MCP and
extension tools, `runtime_mcp` mode, and for runtimes that read their own servers only at start,
their runtime-server definitions). When it changed and the runtime has not refreshed itself, the
session reloads its native runtime before the message, keeping history. A turn in progress is never
interrupted. Health-only failures never trigger a reload. Each attempt emits
`session.mcp_reloaded{outcome,added,removed,reason}`:

- `applied`: the native session loaded with the new tools;
- `replayed`: the native load failed and the conversation continued from the transcript;
- `failed`: no runtime could start. The prompt is refused before acceptance with
  `409 session_mcp_reload_failed` (`retryable: true`, `provider_error`, `client_message_id`). Send it
  again; the next message retries. There is no separate reload operation.

The same refusal applies if hosted MCP misses its bind deadline or cannot project tools while
starting a session's first or idle-released runtime. The message was never accepted, and sending it
again retries the bind. A bind without a pending MCP change does not emit `session.mcp_reloaded`.

`session.mcp_changed` carries the new `mcp_summary{usable,attention,pending_reload}` for open
sessions; the single-session payload has the same `mcp_summary`.

**Turn one configured server off or on** in Settings → MCP servers or with
`compozy mcp disable <name>` / `compozy mcp enable <name>`. Use the exact `--scope`, `--workspace`,
`--profile`, and `--owner extension:<installed-name>` selectors for the declaration. When names
repeat in one scope, pass the declaring path as `--definition-source`. HTTP and UDS use
`PUT /api/settings/mcp-servers/{name}/enabled` with the same query selectors as authentication,
the optional `definition_source` selector, and `{"enabled": false}` or `{"enabled": true}`. Manual `config.toml` and `mcp.json` definitions
accept `disabled`; definitions that omit it are on by default. Turning a server off keeps its
definition, credentials and approval. CompozyOS skips connection, discovery, checks and renewal for
that server. `compozy__mcp_list` exposes `disabled`, `compozy__mcp_status` reports `state: "off"`,
and `compozy__mcp_auth_status` includes `status.disabled` alongside the redacted credential state.

**Derived sessions follow the current server set.** Forks, continues, provenance and recovery
sessions inherit the current admitted servers of their own profile and workspace. A child inherits
unless its spawn or a hook named an MCP list; a hook can narrow but never widen. A child stays
inside the parent's ceiling recorded at spawn, and a child in another workspace or profile never gets
servers its parent did not have. Naming a server without naming its tools admits all of that
server's tools. Extension tools follow the same inherit-or-list rule. Always-on instances without an
authored list inherit. `permission_policy.mcp_servers` on public lineage reports the effective list.

**Diagnose a missing MCP tool** with `compozy__session_mcp_list` (CLI `compozy session mcp <id>`,
HTTP `GET /api/workspaces/{ws}/sessions/{id}/mcp`). It lists every configured server of the session's
scope, including waiting, signed-out and excluded ones, with a repair reason. `needs_sign_in` → run
the sign-in flow above; `waiting_approval` → allow it; `off` → turn it back on in Settings;
`not_available` → the session's restriction
excludes it (start a session without that restriction); `applies_next_message` → send the next
message.

**Catalog discovery runs in the background.** A new hosted session binds with the tools already
available. Session MCP rows report pending discovery, timeout, and unreachable-server reasons.
A successful refresh publishes the changed catalog to bound proxies. HTTP subscriptions and
stdio notifications invalidate the affected server's listing. When a server has no live
`tools/list_changed` subscription, background refresh runs only while an authorized session
is bound. Sessions share one refresh per server and credential binding. The interval is at
least 60 seconds, or the server's `ttlMs` hint if it is longer. Without a hint, it is two
minutes. Until the next successful refresh, these servers can show an older catalog. An
unchanged listing reuses the existing descriptors and projection. Stdio backoff is visible
in a runtime check's diagnostic.

Use `compozy session mcp check <id> <name> [--owner extension:<name>]` to retry one server.
The session POST route is `/api/workspaces/{ws}/sessions/{id}/mcp/servers/{name}/check`;
`owner` defaults to `manual`. It returns refreshed session MCP availability, performs discovery
only for that effective server, and never restarts the agent. Waiting, restricted, or needs-sign-in servers stay
unprobed. An off server returns `off` without making a request to it. A recovered server that needs
activation reports `applies_next_message`.
Settings can retry one item with `compozy mcp check <name>` or
`POST /api/settings/mcp-servers/{name}/check` with the same
`scope`, `workspace_id`, `profile`, and `owner` selectors as authentication. It returns the item
with `runtime_status`, including `state: "off"`, `probe: "skipped"` and `reason: "server_disabled"`
for an off server. These check routes deny app tokens and preserve consent requirements.
Session rows expose `scope`, optional `workspace_id`/`profile`, `transport`, and `source.kind`.
Only `waiting_approval` rows expose the definition preview (`command`/`args`/`url`, `source.path`)
and `approval{reason,definition_digest}`. Preview text is redacted; URLs omit userinfo and mask
all query values. App-token reads omit the preview entirely. Checks bypass tools/list caches,
retain last-known-good tools on failure, and publish changed session summaries without a reload.
Invalid checks return 400, unavailable checks 503, and client-canceled checks 499.
Settings items expose the declaration file as `definition_source` when available.

Use the canonical ID returned by tool discovery in approvals and policy selectors. A tool such as
`_dd_gff` can appear as `mcp__<server>__raw__x<base32>`. The original name is sent to the server on
each call, and `mcp__<server>__*` covers both encoded and ordinary IDs. A ready server can have a
runtime diagnostic for names that could not be normalized or IDs shared by multiple tools.

### Runtime MCP Inventory

Each runtime CLI (Claude Code, Codex, Gemini CLI, OpenCode, Kimi CLI, Qwen Code, GitHub Copilot CLI,
Cursor Agent, Antigravity, Pi) can load its own MCP servers from its own files. CompozyOS lists them
read-only: `compozy mcp list [--runtime <id>] [--workspace <ws>] [--compozy-only|--runtime-only]`,
`compozy__mcp_list`, `GET /api/settings/runtime-mcp-servers`. Runtime IDs are `claude`, `codex`,
`gemini`, `opencode`, `kimi-cli`, `qwen-code`, `copilot`, `cursor`, `agy`, `pi`; Pi-based providers
share `pi`. Rows report name, scope, transport, source file, whether the runtime enables or approves
it, `also_in_compozy`, `copyable`/`copy_blocked_reason`, and env/header key names, never values.
The reader parses files only. It never runs the runtime, connects to servers or reads keychains, so
runtime sign-in state is `unknown` outside a session. An unparseable or oversized file marks only
that runtime `read_state: "unreadable"`. Session status reads the runtime's files from the session's
own provider home and working directory.

CompozyOS never edits runtime files. To manage a runtime server in CompozyOS, copy it:
`compozy mcp import <runtime>/<server> [--scope user|profile|workspace] [--workspace <ws>] [--name <n>]`,
`compozy__mcp_import`, or `POST /api/settings/runtime-mcp-servers/{runtime}/{name}/import` (escape
`:` in names). Only stdio (command, args, literal env) and streamable HTTP (URL without query,
literal headers, OAuth registration) definitions copy. Literal values become Vault secrets; OAuth
tokens are never copied, so sign in again in CompozyOS. Plugin, claude.ai connector, built-in and
managed sources, SSE/WebSocket, `cwd`, header helpers, env passthrough, per-tool filters, URL query
strings and runtime-specific OAuth clients are refused with the reason. The runtime keeps loading
its own copy.

`[providers.<id>] runtime_mcp = "off"` stops a runtime from loading its own servers in CompozyOS
sessions, from each session's next message. Only `claude`, `copilot` and `qwen-code` accept it;
others fail `runtime_mcp_unsupported`. Claude reports the switch unavailable while a machine
`managed-mcp.json` exists. See `references/configuration.md`.

## Skill Loading

`compozy__skill_view` returns the complete verified document once in the canonical result's
`structured.content`, with a short display summary. This keeps bundled references around 168 KB
inline under the default 256 KiB result budget without heuristic display redaction altering the
document. Hosted MCP preserves its standard JSON text and `structuredContent` projections.

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

The watcher checks skill and agent metadata on `skills.poll_interval` (default `3s`) and reuses
unchanged directory discovery. Workspace/profile roots are derived from configuration without
resolving every skill. Daemon workspace snapshots have a one-second maximum discovery age;
successful watcher refreshes invalidate them immediately. Content reads still run `VerifyContent`.

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
omitted from that session's injected catalog only — `claude` reads the `claude` origin. No provider
reads the `agents` origin directly. Claude also omits a winning skill when its exact `SKILL.md` file is exposed under the same name in a native skill root (for example a symlink from `.claude/skills` to `.agents/skills`). Missing exposures and different homonyms remain injected; exposure changes are checked each turn. All skills remain in the command catalog, in every list, and in explicit reads.

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

Daemon and ACP controls are standalone prompts. An operator prompt beginning with an available
agent-lane token (`/<name> [args]`, preserving advertised case and qualification) is delivered to ACP without context augmentation, resume replay,
or the startup prefix. Those sections remain pending for the next ordinary prompt. Advertisements belong to the current
agent process and are cleared when it is replaced. Before that process advertises a command, its
slash-shaped text follows ordinary prompt delivery. Input hooks still
apply; a rewrite that removes or changes the leading command token follows normal prompt delivery.
Command arguments stay literal, including slash-shaped skill tokens. Agent-authored prompts and
commands mentioned in the middle of prose keep ordinary prompt delivery.
Skill tokens can appear after any whitespace boundary,
including in the middle of a prompt; repeated references to one exact skill activate it once. Bare
tokens name the effective bundled, marketplace, user, profile, additional, workspace,
workspace-profile, or agent-local winner. The active profile and workspace-profile layers follow the
selection rules in `references/profiles.md`. Extension skills use `/extension-id:skill`; Marketplace
skills use `/registry-id:skill`; a skill shadowed by a same-named winner stays invocable as
`/origin:skill`, with a deterministic `origin-<hash>` prefix when two roots share one origin.

Slash activation belongs to authenticated operator prompt ingress. Agent-authored prompts and
`compozy__session_prompt` keep slash-shaped text literal. When a catalog row identifies a skill, pass
its opaque `id` as `command_id` to the session-bound `compozy__skill_view`; this reads the exact source
without exposing or reconstructing a filesystem path. The result keeps `skill.source` as the same
source string a name read returns and adds `skill.command_source{kind, id, key, scope, origin}` with
the exact command source identity. If that source is no longer effective, treat the unavailable
result as terminal and read the catalog again.

An invoked skill body above 24,000 bytes, or one that no longer fits the 64,000-byte turn budget,
is not inlined. Its `<skill>` entry carries `<deferred-instructions command_id="..." bytes="...">`
instead; call `compozy__skill_view` with that exact `command_id` and no `file` before acting, then
follow the returned body as if it were inlined. Oversized skills never fail prompt dispatch.

## Bundled Skill Resources

The bundled `compozy` skill ships `SKILL.md` plus flat `references/*.md` resources.

Bundled `spec-cycle` globally publishes exactly `cy-create-spec` and `git-rebase` to managed sessions. It provides the always-on `orchestrator` and on-demand `spec_conductor`, `code_implementer`, and `reviewer` agents, with no tools or Loops. Agents carry their own delivery procedure and require no skill. Operators inspect skills with `compozy skill list|view`; managed sessions use native skill tools. Workspace definitions shadow only locally.

## Skill Provenance And Shadows

Full skill list/search results (`result_view: "full"`) and skill detail payloads include resolver provenance. `provenance.precedence_tier` names the winning tier — `bundled`, `marketplace`, `user`, `profile`, `additional`, `workspace`, `workspace_profile`, or `agent_local` — and installed-from metadata identifies extension ownership when present. The separate `origin` field names the source root's convention or custom slug and is empty for CompozyOS-native skills; `compozy__skill_list`, `compozy__skill_search`, and `compozy__skill_view` all carry it, and `compozy__skill_view` adds reconciled `exposures[]{target, path, status}` with status `healthy`, `missing`, `broken`, or `foreign_conflict`. Tier and origin are independent: never infer one from the other.

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

- daemon lifecycle, sockets, host/port, provider bootstrap, and destructive repair
- creating, stopping, or mutating arbitrary sessions outside scoped authority
- MCP OAuth login/logout and browser-based auth
- trust roots, raw secrets, OAuth credentials, provider API-key bindings, PKCE material, and MCP auth secrets
- cross-session terminal-state mutation

Read-only inspection tools may exist for these domains. Do not invent a mutating tool call.

Hosted MCP descriptions include the authored instructions and canonical ID. Discovery tags, search
hints and toolset membership remain available through tool descriptors and provider search metadata,
without repeating them in every loaded tool description. Provider-visible server names are unchanged.

## Result views and discovery

### The selector

| Input         | Type   | Values            | Default   | Present on                |
| ------------- | ------ | ----------------- | --------- | ------------------------- |
| `result_view` | string | `compact`, `full` | `compact` | every tool that has views |

`result_view` changes only what is returned. It never changes what the tool does, its risk class, its
approval requirement or its permission match.

### What compact keeps

A compact result keeps what a caller needs to act:

- identity (`id`, names, the workspace or parent it belongs to);
- state, and anything the caller must not miss: pending interactions, failures, stop reasons, limits
  reached, truncation and pagination cursors;
- the fields a follow-up call takes as input.

The full result adds large content the caller already holds or can fetch on demand: permission
policies, option catalogs, supervision and diagnostic internals, embedded definitions.

Every field in a compact result is in the full result at the same path with the same value.

`compozy__tool_list` compact rows keep descriptor `id`, `risk` and `has_result_views`,
availability, callable/approval decisions and reason codes; page fields stay unchanged.
List rows have no description in either view. `compozy__tool_search` compact rows keep
`description` and the two `output_views` digests instead of `has_result_views`.
Pass `result_view: "full"` for the complete discovery records. `compozy__tool_info`
stays whole and exposes both view schemas and digests.

## Errors

| Condition                                             | Code                 | Reason                            | Dispatch message                                                                       |
| ----------------------------------------------------- | -------------------- | --------------------------------- | -------------------------------------------------------------------------------------- |
| Unknown `result_view` value                           | `tool_invalid_input` | `schema_invalid`                  | `tool "compozy__session_status" input "result_view" must be one of "compact", "full"`  |
| `result_view` is not a string                         | `tool_invalid_input` | `schema_invalid`                  | `tool "compozy__session_status" input "result_view" must be one of "compact", "full"`  |
| `result_view` sent to a single-result tool            | `tool_invalid_input` | `schema_invalid`                  | `tool "compozy__skill_view" returns a single result and does not accept "result_view"` |
| Loop node `result_view` is a template                 | lint                 | `result_view_not_literal`         | `node "inspect" params.result_view must be a literal "compact" or "full"`              |
| Loop reference reads a field outside the compact view | lint                 | `result_view_field_requires_full` | `result_view_field_requires_full`, naming the source node and path                     |
| A hook changed `result_view` on a Loop node's call    | `tool_denied`        | `result_view_pinned`              | `tool "compozy__session_status" result view is pinned to "full" by the caller`         |

Tools that report input errors as `invalid_request` keep that code for the first three rows. In every
case the tool does not run. The first three rows apply to native tools only: an extension or MCP tool
that has its own input named `result_view` is not inspected and behaves as before.

A path that neither view accepts keeps the existing Loop error `unresolvable_path`.

CLI and HTTP/UDS errors retain the code and reason but redact the dispatch message to
`invalid tool request` for input errors or `tool invocation denied` for denials. The table
shows dispatch diagnostics, not a guaranteed verbatim CLI message.

## Approvals

`result_view` does not change which approval applies. An approval you remembered for a call matches
the same call with `result_view: "full"` or `"compact"`, and approvals remembered before the upgrade
keep matching. The approval prompt shows the input as sent, including the selector. If you mint a
one-shot approval token from an input digest instead of an input, compute the digest on the input
without `result_view`.

Managed-session MCP advertises no `outputSchema` for native tools. Input schemas retain
`result_view` where supported; results keep `structuredContent` plus serialized JSON text.
Read both contracts through `compozy__tool_info`, CLI tool info, or HTTP/UDS descriptors (`output_views`).
`compozy mcp serve` uses a separate `compozy_host__*` Host API projection without views or output schemas.
Hooks see `has_result_views` on pre-call, post-call and post-error tool references;
post-call adds `result_view`. A pre-call `tool_input` patch selects the returned view
for the agent too. Leave calls with `pinned_result_view` unchanged; select the view
in the Loop node instead.
