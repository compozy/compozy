# Extensions

## Contents

- Extension kits
- Packaged inputs and server auth
- Built-in open design
- Portable Agent Plugins
- Marketplace categories and Add
- Install trust
- Built-in permanent removal
- Authoring and dev loop
- Instance scoping
- Dev overlay versus published install
- Reload, last-good, and failure states
- Logs and watch
- Hooks

## Extension Kits

An extension kit is the static resource set shipped by one extension: skills, agents, Loops, automation jobs and triggers, layouts, and MCP sidecars. The manifest owns the paths. Installation enables the kit by default. Per-profile enablement and resource placement decide what is published in each profile.

Declare static paths as tables, for example `[[resources.agents]]` with `path = "agents"` and an
optional `profile`. Previous string arrays are normalized during loading through v0.3.0-beta.30;
builds emit tables. Convert handwritten strings to `{ path = "..." }` entries before their removal
in v0.3.0-beta.31. Omit `profile` to retain the previous all-profile visibility.

Inspect the extension's shipped-versus-live view with `compozy extension inventory <name> -o json`, `GET /api/extensions/{name}/inventory`, or `compozy__extensions_inventory`. Use `--profile <name>` on the CLI; HTTP/UDS accepts `?workspace=<id>&profile=<name>` to inspect a particular instance. Native inventory uses the caller's trusted workspace and profile. Use `POST /api/extensions/preview-install` before installation to review declared profile creation or binding, credential requirements, placements, and any Gateway digest without changing state.

In extension detail, `created_by_extension: true` confirms that this extension created the current
profile ID. `false` covers existing-profile bindings and historical declarations whose creator was
not recorded. An upgrade preserves those markers without inferring authorship or reseeding defaults.

Extensions declare required environment variable names. Bind an existing Vault reference with `compozy extension secrets bind <name> --env <key> --vault-ref <ref> --profile <profile>`, or set a value through stdin or a hidden prompt. Set, bind, list, and unset resolve and transport the selected profile; without `--profile`, they use the normal profile-resolution chain. Add `--remote-header <server>:<header>` to bind that value to one declared remote MCP header. Reads expose bound key, server, and header names only, never values or Vault references.

For client Agent Plugins, the same list and set commands accept optional secret inputs already
referenced by MCP header templates. Use the listed key name and omit `--remote-header`. These
credentials belong to the exact profile and extension instance. An unset input with an empty
default leaves its credential header out of the request.

If a candidate extension changes its normalized gateway permission requirement, install or update returns `extension_gateway_confirmation_required` with the exact digest before changing package state. Inspect that digest and retry with `--confirm-gateway-requirement <digest>` or the equivalent `confirm_gateway_digest` request field. Do not confirm a stale or reconstructed digest. Confirmation records consent to those exact gateway permissions.

A subprocess extension that publishes layouts directly declares the generic Host API permissions and `window_layouts` family. `resources/snapshot` is complete desired state for that extension source, not an append call: advance `source_version`, include every record that remains owned, and let omission delete stale records. Codec, kind, scope, and workspace-binding failure reject the snapshot atomically.

Workspace-bound subprocesses keep that binding when running under a named profile. Domain Host API
calls such as `tasks/create` inherit the workspace and profile; an explicit global scope or foreign
workspace is refused. Resource calls retain their complete `workspace_profile` scope for resource
kernel authorization. A tool approval authorizes the pending command without widening these scopes.

Task create and detail responses preserve `draft` and the saved `wake_creator` setting. Draft tasks
require publication before execution; `wake_creator` describes whether completion should notify
their creator. Omitting that creation option keeps its default of `true`. Fresh detail reads also
carry the persisted event cursor, current run, direct/inherited pause, block reasons and attention
state. Read the fresh detail when following events; the creation result can precede its audit append.

## Packaged Inputs And Server Auth

Native manifests declare install values with `[[inputs]]`: unique `id`, `prompt`,
`type = string|identifier|boolean|secret`, `required`, and `binding = {type, name}`.
An `env` binding must match a server's `env` or `secret_env` key (the map value is the input id),
or name an author-declared `requires_env` entry; the latter delivers the value to the extension
subprocess as that env var and needs `min_compozy_version >= 0.7.9` when no server references it.
SDK-built extensions declare the same inputs with `inputs` (TS) / `Inputs` (Go); `build` writes them
into the generated manifest.
A `url_query` binding must match a parameter declared in a remote server URL. Server-bound secrets
require `secret_env`; secrets cannot bind URLs or have defaults. Boolean defaults are booleans; other
non-secret defaults are strings. Values are NUL-free and at most 8 KiB; identifier values use
URL-safe unreserved characters.

`[resources.mcp_servers.<name>.auth]` accepts `method = "oauth"`,
`registration = "dynamic"`, optional `issuer_url`, and scopes. The issuer must be advertised by
the protected resource. `default_scope` supplies the installation default and never expands an
existing installation. Validate the package with `compozy extension validate <directory>`.

`compozy-catalog publish` derives version, inputs, archive and digest from the package. It emits
v3 extensions and presets only; change `catalog/sources.json` or the package, not generated feed JSON.
The site reads the same v3 catalog. Root feeds and v2 fallback are removed.

Install selectors are `scope = global|workspace`, `workspace_id` (registered ID), and `profile`
(name) in HTTP/UDS/native requests. CLI equivalents are `--scope`, `--workspace <name|path|id>`,
and `--profile`; the explicit flag wins over `COMPOZY_PROFILE`. An operator omitting the profile
keeps all-profile installation. Agents remain bound to their trusted workspace/profile.
With neither scope nor workspace selected, operators use the manifest servers' common default
(`global` when undeclared); mixed defaults require an explicit scope. A workspace default requires
trusted workspace context or an explicit workspace selector. Inputs and vault bindings use that exact cell. A `vault_ref` input must already belong to the same
extension/profile/workspace under `vault:extensions/`; manual MCP references are not imported.

Install typed values with `compozy extension install compozy/supabase --input project_ref=my-project`.
Use `--input-file` for structured value/vault_ref envelopes; existing scoped extension-secret refs
stay in that instance. The CLI derives types from the manifest. Required-field failures include
`inputs` and candidate `input_definitions` in `extension_inputs_required`; prompt for those fields
and retry the same scoped request. Inputs remain isolated by profile: an unconfigured profile
publishes no packaged MCPs and does not block installation or restart in a configured profile.
Subprocess inputs share this gate and the same `inputs`/`input_definitions` lists. Their values,
non-secret ones included, are stored as instance env bindings in the Vault, so
`compozy extension secrets set <name> --env <KEY>` also satisfies them, and `secrets unset` removes
the value for the process and its MCP servers until it is written again; removing the installation
discards it. Launch injects the bound
value or the input default; the daemon environment never reaches the subprocess and never satisfies
a subprocess-only input. A stored value that no longer fits a changed input type reads invalid:
required ones block enable, optional ones are omitted from the process and its MCP servers.
`extension_input_invalid` names the invalid input_id, and
`extension_source_changed` requires reviewing the current artifact before retrying.

Update requests accept the same scope/workspace/profile selectors. CLI `extension update` uses
`--scope`, `--workspace`, and `--profile` (or `COMPOZY_PROFILE`); `--all` filters to that selected
installation scope/profile. HTTP/UDS and `compozy__extensions_update` also accept typed `inputs`
for one named extension. Updates retain every attachment and its creation time; package bytes are
shared, while supplied inputs and rollback before-images address only the selected cell. A workspace
caller cannot mutate an inherited global attachment as though it belonged to the workspace.
Direct GitHub updates retain their acquisition origin and do not depend on the curated catalog.

## Built-in Open Design

The bundled `open-design` extension creates or binds the `open-design` profile. Its default agent
is `open-design-designer`; `open-design-critic` provides independent review. The profile includes
`open-design`, `open-design-review`, `open-design-browser`, and `open-design-setup` skills, the `open-design-review`
Loop, and the read-only `ext__open_design__lint_artifact` tool. Two local references cover craft
and the four artifact formats; there is no upstream library, download, or synchronization step.

Use the designer for interface, site, HTML-slide, or visual-document requests. A short brief or a
`_uiux.md` is valid. Outputs stay under the active workspace's `docs/design/`, with explicit paths
preferred; the user opens HTML directly. This port uses native CompozyOS execution and does not
require OpenDesign's app, daemon, or MCP server. It does not implement production application code.

The designer has two targets. HTML is the default. The library target builds a prototype with the
project's own component library and renders it through the project's toolchain. Its environment is
recorded in `docs/design/prototyping.md`; when that record is missing or failing, the designer
follows `open-design-setup` to inspect the project, reuse or add the smallest prototyping host, get
the user's approval before changing the repository, prove a smoke prototype renders, and write the
record. Library prototypes refresh rendered evidence HTML under `docs/design/` for lint and review.

The lint tool accepts `paths: string[]` and obtains its root from authenticated workspace context.
It requires Node.js, preserves original findings, reports file digests, and never writes artifacts.
Its `passed` means zero original P0 findings. The optional review Loop uses an independent
critic session and verifies the approved file digests before completion. Its optional `target`
input (`auto`, `html`, `library`) binds the designer; a library result must list its prototype sources. Its skill passes per-run
`config_overrides` of `iteration_cap: 3` and `reattempt_strategy: full_body` so runtime defaults
cannot replace the workflow limits. Use the same overrides for direct invocations. The critic considers findings and
specific source-backed exceptions. A normal design request does not start the Loop automatically.
Use its skill and this reference's normal lifecycle surfaces to inspect or operate it.

Browser inspection uses the local skill plus an available agent-browser CLI/browser and an
image-capable harness. Missing browser support is reported, not replaced with a fabricated visual
check. Source attribution and licenses ship with the extension; the selected content is maintained locally.

## Portable Agent Plugins

CompozyOS detects a package layout after source acquisition; there is no format flag. A root
`extension.toml` or `extension.json` selects `compozy`. Root `plugin.json` alone selects
`agent-plugin` and accepts Agent Plugins schema `1.0.0`. When both exist, the native manifest wins
and install records the unused portable manifest as a note. Without a root manifest, `.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`, then
`.cursor-plugin/plugin.json` are tried in that order. Their client grammar adapter loads skills
from `skills/` and declared paths, and MCP servers from a contained `mcpServers` JSON file or
inline map. Authored commands/agents/hooks emit `client_component_ignored` and are not loaded.
When a client manifest omits `mcpServers`, a root `mcp.json` (preferred) or `.mcp.json` is loaded
and reported as `client_mcp_discovered`; a plugin with no manifest loads default skills and root MCP
file under its marketplace entry name. `${CLAUDE_PLUGIN_ROOT}`/`${CURSOR_PLUGIN_ROOT}` alias
`${PLUGIN_ROOT}` and `${CLAUDE_PLUGIN_DATA}` aliases `${PLUGIN_DATA}`. Unsupported server fields are
dropped with `client_field_ignored` instead of rejecting the server. `${NAME}` / `${NAME:-default}`
in a server's `url`, `headers`, `env`, or `args` become typed plugin inputs: required unless
defaulted, secret (vault ref) when the name ends in `KEY`, `TOKEN`, `SECRET`, or `PASSWORD`; bad
names report `client_placeholder_invalid`. A remote server's `auth` (`CLIENT_ID`, `${CLIENT_SECRET}`,
`scopes`) maps to its OAuth configuration.
Package bytes are unchanged; provenance records the selected layout. Layout grants no trust.

Portable ingestion synthesizes a resource-only extension from immediate child skills and `mcp.json`
servers. `stdio` and `streamable-http` map into extension MCP resources; invalid components and `sse`
servers are skipped independently. Install, status, inventory, list, HTTP/UDS, and native-tool reads
carry `format` plus diagnostics. The marketplace `format` badge is display metadata only; acquired
package detection is authoritative.

Portable skill and MCP delivery is verified on managed Claude Code sessions. A provider configured
with `session_mcp = false` receives no per-session MCP servers, so a package that requires hosted MCP
fails before that provider launches.

Recorded ingestion skips use `extension_agent_plugin_component_skipped`. Runtime availability uses
live codes such as `extension_mcp_server_unhealthy`; reads sort ingest diagnostics before live ones so
package validity is never confused with server health. Fatal detection codes are
`extension_agent_plugin_schema_unsupported`,
`extension_agent_plugin_not_manifest`, and `extension_agent_plugin_manifest_invalid`.

`compozy extension validate <path> -o json` takes the portable branch without installing or executing
code and returns `format`, `would_ingest`, and ordered issues. A warning-only report exits zero; a
fatal issue exits nonzero.

Every portable instance receives absolute `PLUGIN_ROOT` and `PLUGIN_DATA` values. The dedicated data
directory is created immediately before its first stdio launch, survives updates, and is deleted on
remove. A failed deletion is renamed out of the reusable instance key and reported as a quarantine
warning; if quarantine also fails, removal fails and the instance remains installed.

## Marketplace Categories And Add

Every catalog entry has exactly one category id: `productivity`, `communication`, `design`,
`development`, `data`, `cloud`, `finance`, `sales-marketing`, `research`, `security`, `ai-agents`,
`more`; `featured` is a reserved section fed only by CompozyOS-curated sources (`compozy-catalog`,
`compozy-plugins`, which honor `x-compozy.category|featured|supersedes`). Listings carry `category`,
`category_label`, and `featured`.

- `compozy marketplace categories [query] -o json` / `compozy__marketplace_categories {query}`:
  taxonomy with counts (human output hides zero counts; JSON keeps them, plus `featured`).
- `compozy marketplace search --category <id>` / `compozy__marketplace_search {category}` /
  `GET /api/marketplace?category=<id>`: one category, paged; the cursor binds the category. An
  unknown id is `marketplace_category_unknown` (CLI exit 2, HTTP/UDS 400, tool error).
- `GET /api/marketplace/sections?per_section=1..12`: the Browse home in one read.
- Add = `compozy extension install <ref> --enable` / `compozy__extensions_install {enable: true}` /
  `POST /api/extensions {"enable": true}`: install and enable in one operation. Every install gate
  still applies (`extension_inputs_required`, consent, conflicts, gateway). If enabling fails after
  install, the result is still success (`201`, exit 0) with `extension.enabled: false` and
  `enable_error {error, code}`; the package stays installed. Retry with
  `compozy extension enable <name>` / `compozy__extensions_enable`.
- Setup state after Add: read input state with `compozy extension setup <name> -o json`,
  `compozy__extensions_setup {name}` or `GET /api/extensions/{name}/inputs`, servers waiting for sign-in via `compozy__mcp_auth_status`, then
  `compozy mcp auth login <server>`. Do not report a plugin as ready from install success alone.
  Each input has `state`: `set` (binding, stored value or default), `missing`, `invalid` (stored value
  fails the current type) or `daemon_env` (an MCP server also consumes it and only the daemon
  environment satisfies it; still ready, but an extension process that uses it does not receive it). Key prompts
  off `state`, never off the `set` boolean, which stays true for `daemon_env`. `missing` lists required
  inputs only; `missing_env` lists bare `requires_env` keys for `secrets set`; `mcp_auth_servers` lists
  OAuth servers needing sign-in. Values and Vault references are never returned.
- Complete inputs with `compozy__extensions_setup {name, inputs}` or `PUT /api/extensions/{name}/inputs`
  (`{"inputs": {...}}`, same value/vault_ref envelopes as install, same scope/workspace/profile
  selectors). Writes merge: omitted inputs keep their value. The result adds `changed` and `reloaded`;
  an enabled instance restarts only when a value changed, a disabled one stays disabled, and the
  installed version, attachments and enablement never change. Only that instance's runtime restarts;
  other profiles, workspaces and extensions keep running. A write that leaves a required subprocess
  input missing still succeeds with `missing` populated and does not restart. A global installation
  serves every workspace from one input cell, so a workspace-scoped write to it is refused; set it up
  with scope global as the operator. The native tool uses the caller's trusted
  workspace and profile and may write only `missing`, `invalid` or `daemon_env` inputs; including a
  `set` input refuses the whole call with `extension_input_overwrite_denied` (`input_ids`), nothing
  written. Changing a set value is operator-only. A call without `inputs` is a read for approvals.
- Operator CLI: `compozy extension setup <name> [--scope global|workspace] [--workspace <ref>]
  [--profile <name>]` prompts on a terminal for required `missing`/`invalid` inputs and every
  `daemon_env` input (Enter keeps a `daemon_env` input unbound and says so); `--all` revisits every input,
  Enter keeping `set` values; `--input id=value`/`--input-file f.json` write without prompting;
  `-o json|toon` never prompts. Exit 0 when nothing required is missing or for any structured read, 2
  when required inputs stay missing in human output or a value is invalid, 1 otherwise. The output ends
  with `Sign in: compozy mcp auth login <server>` when an input feeds an OAuth server. `compozy extension
  install` on a terminal prompts for the inputs an `extension_inputs_required` refusal lists and retries;
  without a terminal or with structured output it keeps the `--input` guidance and exit 2.

Plugin sources accept `url` and `git-subdir` source objects and the Codex index
`.agents/plugins/marketplace.json`; an unreadable shape is reported as `unsupported_source`.

## Install Trust

Starting in v0.6.1, bundled enrollment no longer transfers `dev-cycle` state to `spec-cycle`.
Existing registrations, files, scopes, enablement, inputs and credentials remain intact.
Inspect `compozy doctor` and migrate retained predecessor state before using its suggested
removal command. Doctor names the actual workspace/profile attachment; `--global` does not
remove every workspace attachment. Run v0.6.0 first if automatic compatible-state transfer is needed.

One extension's startup failure remains visible in status without aborting another package's
mutation. If a previous failed install left a package with no scoped installations, an operator
can recover it with `compozy extension remove <name> --global`, then reinstall. A removed built-in
is not recovered by reinstall: restore it with `compozy extension restore <name>` (see Built-in
Permanent Removal). Workspace agents cannot remove an orphaned global package through that recovery path.

Install takes one closed source union — `curated`, `marketplace`, `github`, `git`, or `local_path` — plus a required
`ref` and optional `version`, `asset`, and `allow_unverified`. `compozy extension install <source>`
owns the shorthand: a filesystem path (`./`, `../`, or absolute) becomes `local_path`,
`github:owner/repo[@ref]` and `git:<url>[@ref]` become their named sources, and a bare
`owner/repo[@ref]` tries `curated` first and falls back to `github` only on a `404`. A path that does
not exist fails naming that path instead of degrading into a slug lookup. Git URLs must use HTTPS,
resolve only to public addresses, and carry no credentials, query, or fragment. Git installs require
Git 2.37 or newer so the daemon can pin validated DNS answers; missing Git reports
`extension_git_unavailable`, and an older version reports `extension_git_version_unsupported` (`503`).

Curated refs resolve through the daemon-owned catalog: the runtime downloads the feed-owned artifact
when the entry carries one, verifies the catalog-pinned SHA-256 before extraction, then persists
separate catalog entry, archive digest, and extracted-tree checksum provenance. Official and community
catalog tiers install with no consent. Every other install — curated `unverified` tier, `marketplace`, `github`,
`git`, `local_path` — needs live policy `extensions.trust.allow_unverified` (default `true`) plus the
request-level `--allow-unverified`, which is the whole consent. Policy off returns
`extension_unverified_policy_blocked` with evidence path `/settings/extensions`; policy on without
consent returns `extension_checksum_unverified`. Both are `422`. Human output prompts on
`--allow-unverified` unless `--yes`; structured output requires `--yes`. The deleted key is
`extensions.marketplace.allow_unverified`, and `compozy config set` names its replacement.

Registered plugin sources use `POST /api/extensions` with `source: marketplace`, the listed
`install_slug` as `ref`, and the listed `digest_sha256` as `expected_digest`. Preview and install
retain that same pin. Plugins remain unverified regardless of layout or cache integrity. A cached
package installs without contacting its source; missing/corrupt bytes are acquired again only if the
live package has the same digest. `extension_source_changed` (409) requires a new listing and approval;
`source_unreachable` (503) means the cache is unavailable and the source could not supply the package.
Updates resolve `(source_ref, entry_id)` from installed provenance, so source renames keep identity.
Changed bytes count as an update even when the declared version stays the same. Publication and
rollback use the existing extension lifecycle; shared credentials and other attachments stay intact.

A curated digest mismatch is `extension_archive_digest_mismatch`, terminal for that catalog version
and with no unverified bypass. A GitHub release may carry an `<asset>.sha256` sidecar; when one
exists the daemon verifies the archive against it before extraction and records `digest_matched`.
That fact is integrity only: it never raises `registry_tier` above `unverified`, never sets
`checksum_verified`, and never removes the consent requirement. Any digest failure aborts before the
registry write, so no partial install survives. Registry tier and digest verification are provenance
signals, not safety guarantees. `extension.digest.verify` event queries report `outcome=success` for
matching bytes and `outcome=failure` for mismatches.

Read the persisted decision with `compozy extension provenance <name> -o json`,
`GET /api/extensions/{name}/provenance`, or `compozy__extensions_provenance`; `installed_from` is
`bundled` for an extension shipped with CompozyOS, or `marketplace_registry`, `github`, `git_url`, or
`local_path` for a separately installed extension. Bundled extensions carry the `official` registry
tier and verified checksum evidence without unverified-install consent.

An extension update commits when the registry, managed directory, and runtime reload all succeed.
Post-commit backup or staging cleanup failure does not roll back or relabel that active update:
`status` remains `updated`, and `warnings[]` contains `extension_update_cleanup_failed` with the
cleanup target and residual path. Verify the active version before asking an operator to remove the
residue.

A batch update (`compozy extension update --all`, `POST /api/extensions/update` on HTTP and UDS,
`compozy__extensions_update`) stops at the first failing target without discarding the progress
before it. The response is `200` carrying every completed item plus the failed one, whose `status` is
`failed` and whose `error` carries `extension_update_failed`. Targets after the failure are not
attempted; resolve that item and re-run rather than reading the short list as success. Only a batch
that completed nothing maps to an error status.

Extension removal follows the same commit boundary. After the registry, managed directory, and
runtime reload confirm removal, backup cleanup failure leaves `status` as `removed` and reports
`extension_remove_cleanup_failed` with the residual path. Treat that path as cleanup debt; do not
restore or operate the removed extension from it.

Published reinstall uses the existing update transaction and selected attachment. Repeating the same
acquisition and inputs preserves package and input/secret before-images without a runtime reload.
A different classified `(source_ref, entry_id)` returns `extension_name_conflict` with `installed_origin`;
display names do not participate in identity. Only an operator may associate an unclassified managed
installation with a catalog entry. Publication failure restores the old package and selected input cell.

A local bundle replaces the installed package of the same name with
`compozy extension install <path> --replace`, `POST /api/extensions` with
`{"source": "local_path", "replace": true}`, or `compozy__extensions_install {replace: true}`. The
replacement runs through the same transaction as a published reinstall: the previous directory,
registry record, gateway confirmation, and selected input cell stay available until reload and the
returned status step both succeed, so a failure inside that transaction after publication restores
the previous version. Once the reinstall transaction returns successfully, a subsequent enable
failure retains the new package. `replace` applies to `local_path` only; published
sources already replace implicitly. It never converts a marketplace-owned name — that returns
`extension_name_conflict` with `installed_origin`, so local bytes cannot silently inherit or drop
marketplace provenance or update tracking — and a bundled package is refused with `extension_exists`.
A replacement also does not enroll a new attachment: the target scope must already resolve, and only
an operator may replace a package attached outside the caller's own workspace.

For a configured plugin source, `compozy extension install team/tool --allow-unverified --yes`
selects the source index and pins the listed digest before preview and installation. Existing curated
acquisition refs keep priority if the same spelling collides; `marketplace:team/tool` explicitly selects
the plugin. An unknown explicit marketplace source fails before acquisition; it is never interpreted
as a GitHub repository. Inspect source state with `compozy marketplace sources list -o json`.

### Extension dependencies (experimental)

Dependencies are experimental and may change without a deprecation window. A package declares the
extensions it needs with `[[requires.extensions]]` entries: `name`, `source` (`curated`,
`marketplace`, `github`, `git`, `local_path`), `ref`, and an optional `min_version` lower bound. Self
and duplicate names are invalid, and the manifest must set `min_compozy_version` to at least `0.7.9`
so older daemons refuse it instead of ignoring the table. The catalog feed carries no dependency
field: browsing never shows dependencies; install preview reads them from the downloaded package.

A dependency is met when an installation with that name is visible to the dependent's instance (a
global install counts for workspaces of the same profile, never for another profile), enabled for that
profile, and at least `min_version`. Install and preview resolve the transitive graph before any
mutation: a cycle or a chain deeper than 5 returns `422 extension_dependency_graph_invalid` with
`reason` `cycle` or `depth` and `path`; one name declared with different `source`/`ref` by two parents
returns `reason: "conflicting_source"` with both `parents`. Diamonds collapse to one node whose
`min_version` is the highest bound and whose `required_by` lists every parent.

Preview (`POST /api/extensions/preview-install`) returns `dependencies[]`, dependencies first, each
with `state` (`met`, `missing`, `disabled`, `version_unmet`), `required_by`, `depth`, `registry_tier`
(only `official` and `community` are registry-verified; earlier consent is not), and, for a missing
package, its `version`, `digest_sha256`, `gateway_requirement_digest` and `consent_required` (true when
it needs `allow_unverified`). An installed `disabled` dependency carries `installed_version` and, when
enabling must confirm it again, `gateway_requirement_digest`. A marketplace dependency is previewed at
its listed digest; installing it still requires that digest as `dependencies.<name>.expected_digest`. Install without `with_dependencies` refuses any unmet
dependency with `422 extension_dependencies_required` listing the unmet entries. `version_unmet`
always refuses, even with consent, and carries `retry_command: "compozy extension update <dep>"`;
nothing is ever auto-updated.

`with_dependencies: true` installs `missing` and enables `disabled` dependencies in the dependent's
scope and profile, dependencies first, before the dependent's own input gate. Consent is per
artifact under `dependencies.<name>` (`expected_digest`, `allow_unverified`,
`confirm_gateway_digest`); the root `allow_unverified` never covers a dependency. A missing
confirmation refuses with that gate's existing code (`extension_checksum_unverified`,
`extension_source_changed`, `extension_gateway_confirmation_required`, ...) plus `dependency: "<name>"`.
Execution stops at the first failure and keeps completed dependencies, reported as
`dependencies_installed` on the error (including one committed before its enable failed); a successful
install returns `201` with `dependencies_installed`. A dependency already installed for another scope or
profile is only attached: when its source now provides different bytes the install refuses with
`extension_exists` instead of updating the package other cells use. The dependent's commit re-checks its
dependencies, so one removed concurrently refuses with `extension_dependencies_required`. A dependency with required inputs refuses with `extension_inputs_required`
naming it; set it up separately first. `compozy__extensions_install` accepts the same
`with_dependencies` and closed `dependencies` object with the authority of a plain install.

CLI: on a human TTY, `compozy extension install` answers `extension_dependencies_required` with one
consolidated preview (name, version, source, state, trust, gateway digest, required by), asks once,
then asks each artifact's own gates (unverified consent, gateway digest) and retries with
`with_dependencies`. Without a TTY or with `-o json`, pass `--with-dependencies`; bare
`--allow-unverified` covers only the requested extension, each unverified dependency needs
`--allow-unverified-dependency <name>` (with `--yes` for structured output), and
`--confirm-gateway-requirement` takes a bare digest for the root and `<name>=<digest>` for a dependency.
A `version_unmet` dependency prints `Update first: compozy extension update <dep>` and exits 2.

Remove (`DELETE /api/extensions/{name}`) and disable (`PUT /api/extensions/{name}/enablement` with
`enabled: false`) refuse with `409 extension_dependency_in_use` while an installed dependent still
resolves its dependency to that installation; `dependents[]` lists `name`, `workspace_id` and
`profile` (empty means global or all profiles). `?force=true`, `{"force": true}`, CLI `--force`, or
native `force` on `compozy__extensions_remove`/`compozy__extensions_disable` proceeds. Nothing
cascades: the dependent keeps its runtime state and status/list/info report
`unmet_dependencies: [{name, min_version, state, ...}]` (also in the compact native results and the
human/TOON CLI output). Enabling a dependent whose dependencies are
unmet refuses with `422 extension_dependencies_required`; boot never refuses. The CLI prints the
dependents and exits 1.

## Built-in Permanent Removal

A built-in is a bundled extension, embedded in the binary from `extensions/`. A plain
`compozy extension remove <name>` of a built-in does not stick: the next daemon boot enrolls it again.
`--permanent` keeps it removed. `compozy extension remove <name> --permanent --global`, the HTTP or UDS
call `DELETE /api/extensions/<name>?permanent=true`, and the web dialog's "Remove permanently" choice
delete the same files, rows, installations, enablement, and secret bindings as a plain remove, and write
one tombstone in the same transaction. `--permanent` is global only. A marketplace, local, or dev-linked
row is refused with `extension_not_builtin` (422), an unknown name with `extension_not_found` (404), and a
caller without operator scope with `403`. A built-in that still has other installations, such as an
attachment in another workspace or profile, returns a 409 conflict until those installations are removed.

The tombstone is a row in `extension_builtin_tombstones`, keyed by name. Like bundled rows, it is global to
the daemon home, not per profile. Boot skips a tombstoned name, so the removal survives restarts, upgrades,
and updates. The built-in has no row, so it grants no tool-policy trust and exposes no skills to new
sessions. Plain `remove` is unchanged and still returns at boot.

Restore clears the tombstone and enrolls the bundled version at once, enabled, with no restart:
`compozy extension restore <name>` (`--global` is accepted; built-ins are always global),
`POST /api/extensions/<name>/restore`, or `compozy__extensions_restore` with `{"name": "<name>"}`. It needs
write authority, as `remove` does. Its refusals are `extension_builtin_not_removed` (409) when no tombstone
exists and `extension_exists` (409) when a non-bundled row uses the name. If enrollment fails, the tombstone
is written back and the error is `extension_restore_failed` (500); if the partially enrolled row cannot be removed, that row stays and no tombstone is written. If a permanent removal commits and its compensation then fails, the error is `extension_removal_compensation_failed` (500): the tombstone stays, and `compozy extension restore <name>` brings the built-in back. To list tombstones, use
`compozy extension removed [-o json]`, `GET /api/extensions/removed-builtins`, or
`compozy__extensions_removed_list`; with none, the result is `{"removed":[]}`.

Q2 consequence: an explicit marketplace or local install under a tombstoned name succeeds, and the tombstone
stays. While that operator copy exists, restore returns `extension_exists`, and boot keeps the copy. Removing
the copy with a plain `remove` does not bring the built-in back at boot, because the tombstone still gates
enrollment. Only `compozy extension restore <name>` clears it.

Agents can restore and list. `compozy__extensions_restore` is mutating; `compozy__extensions_removed_list`
is read-only. No tool removes a built-in permanently: `compozy__extensions_remove` takes no `permanent`
input. When a built-in must be removed for good, ask the operator to run the CLI or web action.

## Authoring And Dev Loop

Authoring runs `init` → `build` → `validate` → `dev` → `reload` → `logs` → `publish`. Native tool IDs
and risk flags live in `references/native-tools.md`; what goes in the code lives in
`references/extension-authoring.md`. CLI parity is
`compozy extension init|build|validate|dev|reload|logs|publish`; HTTP/UDS parity is
`POST /api/extensions/dev`, `POST /api/extensions/{name}/reload`, and
`GET /api/extensions/{name}/logs`. Publish has no HTTP/UDS route.

`build` publishes one immutable generation at `<origin>/dist/gen-<hash>`, where `generation_hash` is
the 64-lowercase-hex checksum of that tree. For a code-backed source it compiles and runs SDK describe
mode. For a native resource-only source with at least one declared skill, agent, Loop, automation, or
layout path, it validates the handwritten manifest and copies those trees without running a build or
describe command. A resource-only source cannot use a build-command override or declare a top-level
subprocess, runtime capabilities, Host API permissions, hooks, tools, MCP servers,
command groups, or dynamic resource publication; those contracts require `package.json` or `go.mod`.

That hash is the only generation identity any surface accepts: `dev` takes
`{origin_path, generation_hash}`, `reload` takes `{generation_hash}`, and the daemon reconstructs the
directory, re-verifies the tree digest and manifest, and matches the manifest name before activation.
A malformed, missing, mismatched, or escaping handle returns `400` (`extension: generation is
invalid`); no path, symlink, or staging directory substitutes for it. `validate` is a read-only
manifest, permission, and consent-area report that never executes extension code.

`compozy extension dev` and `compozy extension reload` build locally and send the resulting hash. The
native `compozy__extensions_dev` and `compozy__extensions_reload` never build: call
`compozy__extensions_build` first and pass its `generation_hash`.

`compozy extension publish [generation-directory] --repository <owner/name> --tag <tag> [--draft]`
uploads that generation's archive plus its `<asset>.sha256` sidecar to a GitHub release and returns
the release URL, asset URL, and digest; the directory defaults to the working directory. No surface
accepts a credential field. The CLI reads `GITHUB_TOKEN` from its own process environment, while
`compozy__extensions_publish` resolves `env:GITHUB_TOKEN` then `vault:github/publish` inside the
daemon and registers the value for redaction. An unresolvable credential fails before any upload.

## Instance Scoping

Every runtime extension surface is keyed by instance — extension name plus workspace. The published
installation is the global instance (empty workspace); a dev link is a workspace instance. Subprocess,
operation coordinator, last-good generation, log ring, status, and events are per instance, so two
workspaces linking the same extension share no process, logs, or failure state.

Declared agents, skills, Loops, automations, and layouts use the same instance scope. Within an
instance, resources are visible only when the extension is enabled for the active profile and the
resource is unplaced or placed in that profile. Active dev instances populate only the linked
workspace's catalogs and workspace detail. Reload swaps that workspace snapshot atomically, and
unlinking removes it without mutating the published installation.

The workspace is bound server-side — from the operator's resolved workspace or the agent session's
trusted scope — never from a request body or tool input. An agent caller that names a different
workspace is denied with `403` (`extension: workspace access denied`), and its list, status, logs, and
event projections filter by that workspace. Global-instance logs stay operator-transport-only; reach
them with `compozy extension logs <name> --global`.

CLI `list` and `status` read the published global instance by default. Pass
`compozy extension list --workspace <workspace>` or
`compozy extension status <name> --workspace <workspace>` to inspect the effective workspace instance,
including a dev overlay. The CLI resolves names and paths to the stable workspace registration ID before
calling the existing scoped HTTP/UDS read.

## Dev Overlay Versus Published Install

A dev link is a side-table overlay, not an install. It never mutates or displaces the published row,
and only `dev` creates one. When both exist, reads report `overrides_published: true` beside `dev`,
`origin_path`, `generation_hash`, and `workspace_id`. `compozy extension remove <name>` inside a
workspace unlinks only that overlay and restores the published installation; `--global` removes the
published installation itself. `--permanent` is valid only with `--global` and only for a bundled
built-in, never for a dev overlay (see Built-in Permanent Removal). Dev emits `extension.dev.{linked,unlinked}` and
`extension.reload.{completed,failed}`.

A dev-linked extension is a trusted tool-policy source in the workspace that linked it, so its tools
need no catalog entry, archive digest, or `--allow-unverified` ceremony. Content-hash re-verification
is the integrity boundary for dev instances; Install Trust governs published installs.

## Reload, Last-Good, And Failure States

Link, reload, unlink, and boot activation serialize through one per-instance coordinator. Reload starts
the new generation before retiring the old one. When the new generation fails to activate, the instance
restarts the last-good generation and the call returns the activation error while status reports
`failure_code: "activation_failed"` and `last_error: "activation_failed; running <last-good hash>"`. A
broken edit never takes the extension down; read status before assuming an outage.

At daemon boot, a dev link whose origin no longer exists or now escapes the workspace root loads as
`state: "error"` with `failure_code: "missing_origin"` instead of failing boot. Origin paths are
canonicalized — symlinks resolved, containment enforced under the workspace root — at link time and on
every load. `reload` or `logs` for a name with no overlay returns `409` (`extension: extension is not
dev linked`).

## Logs And Watch

Each instance feeds a bounded 256 KiB drop-oldest ring from subprocess stderr, redacted at ingestion so
no transport sees raw secrets. A snapshot carries an opaque `stream_epoch`; every entry carries that
epoch plus its monotonic `sequence`, `timestamp`, `message`, and `generation_hash`. Resume only with the
pair `after: <sequence>` and `stream_epoch: <epoch>` (CLI: `--after` plus `--stream-epoch`). The SSE
stream publishes `extension_log` deltas and an atomic `extension_log_reset` snapshot when the daemon
recreates the ring. Replace retained rows on reset, including an empty reset. The ring survives reloads
because it belongs to the instance, but it is live retention rather than durable history: a dropped
oldest entry is gone, and unlink/relink starts a new epoch.

For `compozy extension logs --follow -o jsonl`, delta lines are log entries. A reset line has
`event: "extension_log_reset"`, `stream_epoch`, and the complete replacement `logs` array; process an
empty array too.

`compozy extension dev --watch` closes the loop client-side. It polls the source tree every
`extensions.dev.watch_interval` (default `2s`), skips `.git`, `dist`, and `node_modules`, and rebuilds
plus reloads one change at a time. There is no daemon-side watcher.

## Hooks

Hooks are typed dispatch at the owning state transition. They are not a generic event bus and must not tail event/log tables to infer work.

For `tool.*` hooks, match the canonical `tool_id` (for example `compozy__workspace_info`) and read
that field from the payload. Use `tool_input` for an input patch. `tool_name` is a permission-event
matcher; tool events do not accept it or `tool_namespace`.

Hooks may deny, narrow, annotate, or observe. They must not bypass safety primitives such as claim tokens, leases, TTL, lineage, spawn caps, or permission narrowing.

Skill-declared hooks are part of the skill contract. Keep hook declarations structured and validated, not buried in prose.

The `context.pre_compact` and `context.post_compact` hooks are observation-only and match on
`compaction_trigger` (`requested` or `agent`); see `references/runtime-operations.md`. An `AGENT.md` hook
cannot target them, because the agent definition adds `matcher.agent_name` and the context family does
not accept it; declare context hooks in `config.toml`, an extension manifest, or a skill. The retired
matcher keys `compaction_reason` and `compaction_strategy` are rejected in v0.6.1 in every
declaration source. Use `compaction_trigger`; invalid sources remain unchanged.

Manage hooks with `compozy__hooks_*` (list/info/events/runs/create/update/delete/enable/disable). Workspace-scoped declarations match the registered workspace ID shown by `compozy workspace info`, the same ID carried by their event payloads. Hook families are documented beside their domain: `loop.*` in `references/loops.md`, and `window_manager.*` in `references/window-management.md`.

The `agent.heartbeat.wake.before` hook accepts `cancel: true` for proactive `timer` and
`instance_check` sources. Legacy `scheduler`, `manual`, and `harness_reentry` sources ignore
cancellation. Dry-run checks skip this hook. Cancellation creates no triage occurrence.
Heartbeat status includes both proactive sources in recent wake events, preserving their
check results and reasons, including retained events written before upgrade.

For proactive wake cancellation, hooks may return `{"cancel":true,"reason":"maintenance_window"}`.
The persisted wake event keeps `hook_cancelled` as its reason and records the supplied text in
`detail`, redacted and bounded to 300 bytes. An omitted reason still cancels the check.
