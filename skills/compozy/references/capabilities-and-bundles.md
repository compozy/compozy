# Capabilities And Bundles

## Contents

- Capability vocabulary
- Extensibility surfaces
- Cross-surface impact audit
- Agent manageability
- Bundles
- Extension install trust
- Extension authoring and dev loop
- Hooks
- Config lifecycle
- Settings apply lifecycle

## Capability Vocabulary

The canonical Compozy artifact name is capability. Do not use recipe, workflow, procedure, or playbook for current Compozy behavior unless quoting historical material.

A capability should be discoverable, manageable by agents, and represented through public runtime surfaces. It is incomplete if it only works through internal Go calls or the web UI.

## Extensibility Surfaces

When adding or changing Compozy behavior, decide which surfaces are affected:

- extensions and extension resources
- hooks
- skills and capabilities
- tools and toolsets
- bundles
- registries
- bridge SDKs
- MCP sidecars
- CLI, HTTP, and UDS APIs
- docs and generated references

No-impact is acceptable only when there is evidence.

## Cross-Surface Impact Audit

For any feature, bug fix, refactor, contract/API/CLI/native-tool/config/docs update, or runtime behavior change, record the Compozy impact decision before claiming the change complete:

- Native tools: tool IDs, toolsets, descriptors, input/output schemas, schema digests, risk flags, availability diagnostics, capability gates, and agent CLI/API fallbacks.
- Extensibility and hooks: extension resources, hook taxonomy and dispatch call sites, skills/capabilities, tools/resources, bundles, registries, bridge SDKs, MCP sidecars, config lifecycle, docs, and tests.
- Workspace data isolation: whether data is global, workspace-scoped, session-scoped, or agent-scoped; `workspace_id` flow through CLI/HTTP/UDS/core/store/web/SSE/cache/events; and cross-workspace leak tests for list/read/cache/event paths.
- Official Compozy skill: `skills/compozy/SKILL.md` and `skills/compozy/references/*.md` guidance that must change when public behavior or agent-operable surfaces change.

Use `no impact` only with checked-surface evidence. QA/worktree isolation and workspace data isolation are separate decisions.

## Agent Manageability

Every user-visible runtime capability needs an agent-operable path:

- CLI with -o json or -o jsonl where relevant
- HTTP/UDS parity when state crosses the daemon boundary
- discoverable status/config output
- deterministic errors and reason codes
- docs that describe the agent path

UI-only management is incomplete.

## Bundles

Bundles activate related runtime resources together. Treat bundle projection as daemon-owned state. Do not make a bundle depend on prompt prose for authority.

Bundle activation reads expose `version`. Before confirming a changed Live requirement, read the activation with `compozy bundle get <id> -o json`, then pass that value to `compozy bundle update <id> --expected-version <version> --confirm-network-requirement -o json`. A `409 Conflict` means the activation changed; reread it and inspect the current digest instead of retrying with a stale version.

When changing bundle behavior, update resources, registries, config docs, CLI/API surfaces, and tests in the same change. Greenfield Compozy favors hard cuts over compatibility bridges.

Activation list and detail payloads expose `spec_drift` by comparing the stored activation spec hash with the current installed bundle profile. Use `compozy bundle list -o json` or the activation API to inspect it. Reapply with `compozy bundle update <activation-id> -o json`; a successful reapply reconciles current resources, stores the current hash, and clears drift. Activation timestamps are informational and never signal bundle updates.

Bundle profiles can package declarative window layouts by path:

```toml
[[profiles.layouts]]
path = "layouts/two-up.json"
```

Each file is one strict `window_layout` resource JSON document. The path must remain inside the extension root after symlink resolution. Extension load validates the authored resource; preview and activation validate it again in the target scope. Materialization derives an activation-scoped record/spec ID, exposes the layout in activation payloads and inventory, and removes only that activation's owned record during reconciliation or deactivation. Never copy the authored ID directly into storage or bypass the canonical codec.

A subprocess extension that publishes layouts directly must declare the generic Host API methods, resource capabilities, and family:

```toml
[actions]
requires = ["resources/list", "resources/get", "resources/snapshot"]

[security]
capabilities = ["resources.read", "resources.write"]

[resources.publish]
families = ["window_layouts"]
max_scope = "workspace"
```

`window_layouts` grants only kind `window_layout`. `max_scope = "workspace"` requests workspace-scoped publication; use `global` only when the source must also publish global layouts. Source tier, operator policy, and runtime session may narrow it further. `resources/snapshot` is complete desired state for that extension source, not an append call: advance `source_version`, include every record that remains owned, and let omission delete stale records. Codec, kind, scope, and workspace-binding failure reject the snapshot atomically. The generic Go/TypeScript Host API resource record is the SDK contract; bridges and MCP sidecars receive no special layout mutation path.

## Extension Install Trust

Install takes one closed source union — `curated`, `github`, `git`, or `local_path` — plus a required
`ref` and optional `version`, `asset`, and `allow_unverified`. `compozy extension install <source>`
owns the shorthand: a filesystem path (`./`, `../`, or absolute) becomes `local_path`,
`github:owner/repo[@ref]` and `git:<url>[@ref]` become their named sources, and a bare
`owner/repo[@ref]` tries `curated` first and falls back to `github` only on a `404`. A path that does
not exist fails naming that path instead of degrading into a slug lookup, and a git URL carrying
embedded credentials is rejected before any request. Git installs shallow-clone with the `git`
executable; a missing binary is the deterministic `extension_git_unavailable` `503`.

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
`marketplace_registry`, `github`, `git_url`, or `local_path`.

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

## Extension Authoring And Dev Loop

Authoring runs `init` → `build` → `validate` → `dev` → `reload` → `logs` → `publish`. Native tool IDs
and risk flags live in `references/native-tools.md`. CLI parity is
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

### Instance Scoping

Every runtime extension surface is keyed by instance — extension name plus workspace. The published
installation is the global instance (empty workspace); a dev link is a workspace instance. Subprocess,
operation coordinator, last-good generation, log ring, status, and events are per instance, so two
workspaces linking the same extension share no process, logs, or failure state.

The workspace is bound server-side — from the operator's resolved workspace or the agent session's
trusted scope — never from a request body or tool input. An agent caller that names a different
workspace is denied with `403` (`extension: workspace access denied`), and its list, status, logs, and
event projections filter by that workspace. Global-instance logs stay operator-transport-only; reach
them with `compozy extension logs <name> --global`.

### Dev Overlay Versus Published Install

A dev link is a side-table overlay, not an install. It never mutates or displaces the published row,
and only `dev` creates one. When both exist, reads report `overrides_published: true` beside `dev`,
`origin_path`, `generation_hash`, and `workspace_id`. `compozy extension remove <name>` inside a
workspace unlinks only that overlay and restores the published installation; `--global` removes the
published installation itself. Lifecycle events are `extension.dev.linked`, `extension.reloaded`, and
`extension.dev.unlinked`.

A dev-linked extension is a trusted tool-policy source in the workspace that linked it, so its tools
need no catalog entry, archive digest, or `--allow-unverified` ceremony. Content-hash re-verification
is the integrity boundary for dev instances; Extension Install Trust governs published installs.

### Reload, Last-Good, And Failure States

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

### Logs And Watch

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

Loop lifecycle hooks use the `loop.*` family for generation, gate, node-terminal, and terminal call sites. `loop.generation.pre` and `loop.gate.pre` are sync control hooks; node and terminal wake behavior is daemon-owned and fail-open.

Network participation exposes `network.participation.pre_resolve` for deny-or-narrow control and
`network.participation.resolved` for post-commit observation. The runtime derives whether a patch
widens intent; a hook cannot self-authorize enrollment. Match either event by `workspace_id`,
`channel`, `participation_mode`, or `participation_source`.

Window-management topology exposes four async observation hooks after a durable commit:
`window_manager.layout.applied`, `window_manager.desktop.created`,
`window_manager.desktop.deleted`, and `window_manager.window.moved`. They carry the workspace,
revision, semantic command, changed entity IDs, actor, origin, and occurrence time. Preview, no-op,
rejected, presentation-only, and route-navigation commands do not dispatch this hook family.

Declarative `window_layout` resources are data-only topology templates. They are strict, versioned,
workspace-bound when scoped locally, and support `any`, `landscape`, or `portrait` aspect variants
plus explicit participant slots and overflow policy. Discover them through the generic resource
catalog, then apply them through the same preview, revision/CAS, validation, commit, event, and
history pipeline as an inline `layout.arrange` command. A resource cannot execute code or receive
pointer events.

## Config Lifecycle

Any feature or refactor must state whether config.toml keys, defaults, docs, and examples are added, changed, or removed. In greenfield alpha, delete obsolete config paths instead of creating aliases or fallback bridges.

If a rename touches code, storage, APIs, CLI, extensions, specs, docs, and task artifacts, update them together.

`[marketplace.catalog]` controls Compozy's curated MCP server, extension, and skill feed projection.
`base_url` defaults to the public `compozy/compozy` catalog on `main`, `ttl` defaults to `1h`, and
`timeout` defaults to `10s`; all three paths apply live to the next fetch. Use the structured config
surfaces plus `compozy config reload -o json` and apply history to change or verify them. These keys do
not replace the independent `skills.marketplace.*` feed settings or the `extensions.trust.*` and
`extensions.sources.*` distribution settings. `extensions.trust.allow_unverified` applies live; every
other `extensions.*` path is restart-required.

`[autonomy.scheduler]` tunes the mechanical scheduler's convergence escalation ladder for starved runs. Keys are wake-cycle counts that must stay positive and monotonic (`fan_out_after` ≤ `spawn_after` ≤ `event_after` ≤ `needs_attention_after`) plus a `min_queued_age` duration. Defaults: `fan_out_after = 2`, `spawn_after = 4`, `event_after = 6`, `needs_attention_after = 10`, `min_queued_age = "2m"`. Validation rejects non-monotonic or non-positive values at load.

These thresholds apply only to true convergence episodes. Compatible sessions that are starting, prompting, processing another run, or reserved earlier in the scheduler cycle hold serial backlog without consuming the ladder. Policy remains serial: saturation does not start extra task-role capacity.

`[loops.defaults.delivery]` and `[loops.defaults.watch]` seed new loop effective config before per-loop `loop_config` overrides; they are desired-state defaults, not the DB-backed override plane. Delivery defaults are `iteration_cap = 50`, `no_progress.window = 3`, `gates.max_revisions = 10`, `budget.tokens = 0`, `budget.wall_clock_sec = 0`, `budget.on_exceeded = "halt"`, and `fan_out_width = 4`. Watch defaults are `iteration_cap = 0`, `no_progress.window = 2`, `budget.tokens = 0`, `budget.wall_clock_sec = 0`, `budget.on_exceeded = "halt"`, and `fan_out_width = 2`; gate revisions remain unset for watch unless configured. Both default families accept field-merged `runtime_defaults.worker|judge.{provider,model,reasoning}` and ordered `runtime_rules` that match one task `id`, `type`, or `complexity`. Operator config may tighten the compile-time ceilings but must not exceed fan-out `64`, no-progress window `30`, or gate revisions `64`. `budget.on_exceeded` accepts only `halt` or `escalate`. These paths are restart-required config lifecycle entries; use `compozy config reload -o json` and apply history to inspect activation.

`[goals]` sets `max_turns = 20` and `context_nudge_ratio = 0.8` for new Goals, plus the daemon-wide durable session-event relay controls `outbox_batch_size = 50` and `outbox_poll_interval = "100ms"`. The Goal defaults are global/workspace-overridable; relay controls use global config because one relay serves every workspace. All four are agent-mutable, restart-required paths. `max_turns` must be positive; the ratio accepts `0.0` through `1.0`, with zero preserved; the relay batch accepts `1` through `200`; and its poll interval must be positive. Each Run pins its resolved ratio and every Goal checkpoint copies that value, so config reload or daemon restart cannot change an active Goal. Relay settings take effect when the daemon starts.

Loop observability is durable runtime state, not a transient UI stream. `loop_run_events` persists replayable workspace-scoped events for status changes, node running/terminal outcomes, gate verdicts, generation starts, channel messages, token ticks, and needs-approval pauses. Payloads are redacted and bounded before persistence; token ticks preserve only usage counters and terminal markers.

Automation schedule catch-up policy is part of the public schedule contract. Recurring schedules accept `skip_missed`, `coalesce`, `replay`, and `run_once_on_catchup`; one-time `at` schedules reject catch-up fields. Omit the policy for the target-aware default: Loop targets with a `watch-source` use `coalesce`, while other scheduled targets use `skip_missed`. `misfire_grace_seconds=0` uses the daemon jitter grace. Durable canceled runs identify `misfire_grace_exceeded` and `self_overlap` under `metadata.reason`. Catch-up starts carry structured automation-run metadata so agents can distinguish normal starts from recovered starts and reason about `concurrency: forbid|queue` outcomes.

`[session.compaction]` controls pressure-triggered checkpoint coverage and replay archiving. Defaults
are `enabled = true`, `pressure_threshold = 0.85`, `max_attempts_per_turn = 1`, and
`failure_cooldown = "10m"`; threshold zero disables admission. All paths are available through
`compozy config set` and the native config tools, are restart-required under the canonical `session.*`
lifecycle rule, and do not mutate the policy bound to the running daemon.

`roles.auto_title.enabled` defaults to `true` and gates the daemon-owned title pass for unnamed user
sessions after their first persisted assistant response. The remaining `roles.auto_title.*` fields
select its agent, provider, model, reasoning effort, and ordered fallback routes. Role changes are
Live desired state for later invocations at global or workspace scope. Explicit names win; disabled
or failed generation leaves the session unnamed.

`[window_manager]` controls global behavior defaults for new-window placement, small-viewport
fallback, focus and raise policy, drag-away grouping, bounded history, desktop transitions, gaps,
snap thresholds and repeat ratios, edge bindings, and shortcuts. Every `window_manager.*` path is
live-applied only after the complete candidate validates; a failed apply keeps the prior active
generation. Workspace topology overrides remain part of revisioned layout documents rather than a
second Settings scope.

## Settings Apply Lifecycle

`config.toml` is desired state. Runtime truth advances only when `ConfigApplyService` applies that desired change to the daemon active generation or records why it cannot.

Agent-manageable settings changes must surface lifecycle status, not just file writes. The public contract names are:

- `SettingsApplyTargetName`: `general`, `memory`, `skills`, `automation`, `network`, `observability`, `hooks-extensions`, `window-manager`, `providers`, `mcp-servers`, `sandboxes`, and `hooks`.
- `SettingsMutationBehavior`: `applied_now`, `restart_required`, or `action_trigger`.
- `SettingsApplyLifecycle`: `live`, `live-add`, `live-remove-if-unused`, `restart-required`, or `session-rebind`.
- `ConfigApplyStatus`: `pending_apply`, `applied`, `blocked`, or `failed`.
- `SettingsApplyNextAction`: `none`, `restart-daemon`, `new-session`, or `retry`.

Use `compozy config reload -o json` to reconcile edited desired state with the active generation. Use `compozy config apply-history -o json` or `GET /api/settings/apply` to inspect persisted apply records. A settings write is incomplete if agents cannot see whether it applied live, requires a daemon restart, affects only new sessions, or failed with retryable diagnostics.

Codegen owns the lifecycle matrix documentation. When config lifecycle rules change, update the source matrix and run `make codegen`; do not hand-edit generated lifecycle docs.
