# Extensions

## Contents

- Extension kits
- Portable Agent Plugins
- Install trust
- Authoring and dev loop
- Instance scoping
- Dev overlay versus published install
- Reload, last-good, and failure states
- Logs and watch
- Hooks

## Extension Kits

An extension kit is the static resource set shipped by one extension: skills, agents, Loops, automation jobs and triggers, layouts, and MCP sidecars. The manifest owns the paths. Installation enables the kit by default. Per-profile enablement and resource placement decide what is published in each profile.

Inspect the selected profile's shipped-versus-live view with `compozy --profile <profile> extension inventory <name> -o json`, `GET /api/extensions/{name}/inventory?profile=<profile>`, or `compozy__extensions_inventory`. Use `POST /api/extensions/preview-install` before installation to review declared profile creation or binding, credential requirements, placements, and any Network digest without changing state.

Extensions declare required environment variable names. Bind an existing Vault reference with `compozy --profile <profile> extension secrets bind <name> --env <key> --vault-ref <ref>`, or set a value through stdin or a hidden prompt. Add `--remote-header <server>:<header>` to bind that value to one declared remote MCP header. List and unset bindings through CLI or HTTP/UDS. Reads expose bound key, server, and header names only, never values or Vault references. Resolution is `(profile, workspace) → (profile, published) → (shared, workspace) → (shared, published)`.

If a candidate extension changes its normalized Network Live requirement, install or update returns `extension_network_confirmation_required` with the exact digest before changing package state. Inspect that digest and retry with `--confirm-network-requirement <digest>` or the equivalent `confirm_network_digest` request field. Do not confirm a stale or reconstructed digest. Confirmation records consent to the requirement; it does not enroll an execution into Live participation.

A subprocess extension that publishes layouts directly declares the generic Host API permissions and `window_layouts` family. `resources/snapshot` is complete desired state for that extension source, not an append call: advance `source_version`, include every record that remains owned, and let omission delete stale records. Codec, kind, scope, and workspace-binding failure reject the snapshot atomically.

## Portable Agent Plugins

CompozyOS detects a package layout after source acquisition; there is no format flag. A root
`extension.toml` or `extension.json` selects `compozy`. Root `plugin.json` alone selects
`agent-plugin` and accepts Agent Plugins schema `1.0.0`. When both exist, the native manifest wins
and install records the unused portable manifest as a note. Client-specific layouts are rejected.

Portable ingestion synthesizes a resource-only extension from immediate child skills and `mcp.json`
servers. `stdio` and `streamable-http` map into extension MCP resources; invalid components and `sse`
servers are skipped independently. Install, status, inventory, list, HTTP/UDS, and native-tool reads
carry `format` plus diagnostics. The marketplace `format` badge is display metadata only; acquired
package detection is authoritative.

Portable skill and MCP delivery is verified on managed Claude Code and Hermes sessions. OpenClaw's
ACP bridge rejects per-session MCP configuration, so a package that requires hosted MCP fails before
provider launch; direct Agent Plugins support inside OpenClaw is a separate path.

Recorded ingestion skips use `extension_agent_plugin_component_skipped`. Runtime availability uses
live codes such as `extension_mcp_server_unhealthy`; reads sort ingest diagnostics before live ones so
package validity is never confused with server health. Fatal detection codes are
`extension_agent_plugin_client_layout`, `extension_agent_plugin_schema_unsupported`,
`extension_agent_plugin_not_manifest`, and `extension_agent_plugin_manifest_invalid`.

`compozy extension validate <path> -o json` takes the portable branch without installing or executing
code and returns `format`, `would_ingest`, and ordered issues. A warning-only report exits zero; a
fatal issue exits nonzero.

Every portable instance receives absolute `PLUGIN_ROOT` and `PLUGIN_DATA` values. The dedicated data
directory is created immediately before its first stdio launch, survives updates, and is deleted on
remove. A failed deletion is renamed out of the reusable instance key and reported as a quarantine
warning; if quarantine also fails, removal fails and the instance remains installed.

## Install Trust

Install takes one closed source union — `curated`, `github`, `git`, or `local_path` — plus a required
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
catalog tiers install with no consent. Every other install — curated `unverified` tier, `github`,
`git`, `local_path` — needs live policy `extensions.trust.allow_unverified` (default `true`) plus the
request-level `--allow-unverified`, which is the whole consent. Policy off returns
`extension_unverified_policy_blocked` with evidence path `/settings/extensions`; policy on without
consent returns `extension_checksum_unverified`. Both are `422`. Human output prompts on
`--allow-unverified` unless `--yes`; structured output requires `--yes`. The deleted key is
`extensions.marketplace.allow_unverified`, and `compozy config set` names its replacement.

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
subprocess, runtime capabilities, Host API permissions, hooks, tools, MCP servers, bridge metadata,
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
published installation itself. Dev emits `extension.dev.{linked,unlinked}` and
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

Hooks may deny, narrow, annotate, or observe. They must not bypass safety primitives such as claim tokens, leases, TTL, lineage, spawn caps, or permission narrowing.

Skill-declared hooks are part of the skill contract. Keep hook declarations structured and validated, not buried in prose.

Manage hooks with `compozy__hooks_*` (list/info/events/runs/create/update/delete/enable/disable). Hook families are documented beside their domain: `loop.*` in `references/loops.md`, `network.participation.*` in `references/network.md`, and `window_manager.*` in `references/window-management.md`.
