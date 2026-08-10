# Extensions

## Contents

- Extension kits
- Install trust
- Authoring and dev loop
- Instance scoping
- Dev overlay versus published install
- Reload, last-good, and failure states
- Logs and watch
- Hooks

## Extension Kits

An extension kit is the static resource set shipped by one extension: skills, agents, Loops, automation jobs and triggers, layouts, and MCP sidecars. The manifest owns the paths. Installation keeps the kit inert; enabling the extension publishes its resources, and disabling it removes only resources owned by that extension instance.

Inspect the shipped-versus-live view with `compozy extension inventory <name> -o json`, `GET /api/extensions/{name}/inventory`, or `compozy__extensions_inventory`. Preview enable with `compozy extension preview <name> -o json`, its HTTP/UDS route, or `compozy__extensions_preview`; the result names added, changed, and removed resources. Inventory and preview are reads; they never publish resources.

Extensions declare required environment variable names. Bind an existing Vault reference with `compozy extension secrets set <name> <key> --vault-ref <ref>`, or enter a value through stdin or a hidden prompt. List and unset bindings through CLI or HTTP/UDS. Reads expose bound key names only, never values or Vault references. Bindings are scoped to the extension instance.

If a candidate extension changes its normalized Network Live requirement, enable or update returns `extension_network_confirmation_required` with the exact digest before changing active state. Inspect that digest and retry with `--confirm-network-digest <digest>` or the equivalent `confirm_network_digest` request field. Do not confirm a stale or reconstructed digest. Confirmation records consent to the requirement; it does not enroll an execution into Live participation.

A subprocess extension that publishes layouts directly declares the generic Host API permissions and `window_layouts` family. `resources/snapshot` is complete desired state for that extension source, not an append call: advance `source_version`, include every record that remains owned, and let omission delete stale records. Codec, kind, scope, and workspace-binding failure reject the snapshot atomically.

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

`build` compiles source, runs SDK describe mode, and publishes one immutable generation at
`<origin>/dist/gen-<hash>`, where `generation_hash` is the 64-lowercase-hex checksum of that tree.
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
no transport sees raw secrets. Entries carry a monotonic `sequence`, `timestamp`, `message`, and the
`generation_hash` that produced them, and the ring survives reloads because it belongs to the instance
rather than the generation. Page forward with `after: <sequence>`;
`GET /api/extensions/{name}/logs?follow=1` streams the same entries as SSE named event `extension_log`,
which `compozy extension logs <name> --follow` consumes. The ring is live retention, not durable
history: a dropped oldest entry is gone.

`compozy extension dev --watch` closes the loop client-side. It polls the source tree every
`extensions.dev.watch_interval` (default `2s`), skips `.git`, `dist`, and `node_modules`, and rebuilds
plus reloads one change at a time. There is no daemon-side watcher.

## Hooks

Hooks are typed dispatch at the owning state transition. They are not a generic event bus and must not tail event/log tables to infer work.

Hooks may deny, narrow, annotate, or observe. They must not bypass safety primitives such as claim tokens, leases, TTL, lineage, spawn caps, or permission narrowing.

Skill-declared hooks are part of the skill contract. Keep hook declarations structured and validated, not buried in prose.

Manage hooks with `compozy__hooks_*` (list/info/events/runs/create/update/delete/enable/disable). Hook families are documented beside their domain: `loop.*` in `references/loops.md`, `network.participation.*` in `references/network.md`, and `window_manager.*` in `references/window-management.md`.
