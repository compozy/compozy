# Configuration

## Contents

- Desired state and apply lifecycle
- Profile layers and credentials
- Host update cadence
- Gateway
- Marketplace catalog
- Skill sources
- Autonomy scheduler
- Loop defaults and observability
- Goals
- Profile selection environment
- Automation schedules
- Session compaction
- Session attachments
- Auto-title role
- Window manager
- Shell session preferences

## Desired State And Apply Lifecycle

`config.toml` is desired state. Runtime truth advances only when the daemon applies that desired change to the active generation or records why it cannot. Effective configuration merges user → personal profile → workspace → workspace named profile. The last layer wins and binds only when its directory name matches the active profile.

Settings changes surface lifecycle status, not just file writes. The public contract names are:

- `SettingsApplyTargetName`: `general`, `memory`, `skills`, `automation`, `network`, `gateway`, `observability`, `hooks-extensions`, `window-manager`, `shell`, `providers`, `mcp-servers`, `sandboxes`, and `hooks`.
- `SettingsMutationBehavior`: `applied_now`, `restart_required`, or `action_trigger`.
- `SettingsApplyLifecycle`: `live`, `live-add`, `live-remove-if-unused`, `restart-required`, or `session-rebind`.
- `ConfigApplyStatus`: `pending_apply`, `applied`, `blocked`, or `failed`.
- `SettingsApplyNextAction`: `none`, `restart-daemon`, `new-session`, or `retry`.

Use `compozy config reload -o json` to reconcile edited desired state with the active generation. Use `compozy config apply-history -o json` or `GET /api/settings/apply` to inspect persisted apply records. A settings write is incomplete until you can see whether it applied live, requires a daemon restart, affects only new sessions, or failed with retryable diagnostics.

Read and write scalar keys with `compozy config show|list|get|set|unset|diff|path` or the `compozy__config_*` native tools. Resolve the live `compozy__config_set` descriptor before mutating: it names the key's scope, lifecycle, and validation. Structured values (arrays, route tables) are edited through `config.toml` or the typed Settings APIs, never guessed into a scalar write.
`compozy__config_get` reports an absent key as `config_path_not_found: config path not found`; after `compozy__config_set`, read the same path again and confirm its structured value.

## Profile Layers And Credentials

`default` writes `$COMPOZY_HOME/config.toml`; a non-default active profile writes
`$COMPOZY_HOME/profiles/<name>/config.toml`. Override with `--scope user|profile|workspace`. The
repository layer `<workspace>/.compozy/profiles/<name>/config.toml` is read-only. A successful lower
layer write may return `ok_overridden`; inspect `winning_layer` before claiming the value is active.

Profile overlays reject `http`, `daemon`, `log`, `database`, `gateway`, `shell`, `marketplace`,
`observability`, `network`, `sandboxes`, and `window_manager.global_shortcuts` with
`profile_config_key_denied`; write machine-only keys with `--scope user`.

Write profile credentials with `compozy --profile <name> secret set
providers/<provider>/<slot> --value-stdin` or the equivalent `extensions/<extension>/<key>` path.
Non-default profiles use `vault:profiles/<name>/...`; `--from-env` fails with
`profile_secret_env_forbidden` because the process environment is shared. Verify only redacted source
metadata with `provider inspect`. For a non-default profile, `secret rm` falls back to the user
credential and requires `--yes` for non-interactive removal when the profile owns work.

## Host update cadence

`[app] update_check` defaults to `true`; `update_check_interval` defaults to `6h` and accepts
`15m` through `168h`. The daemon is the sole consumer and schedules read-only runtime and app
checks. The shell reads no update config. Read the host-global operation with
`GET /api/settings/update`; mutate it with `POST /api/settings/update/apply` using a non-empty
`targets` array (runtime first), or through
`POST /api/settings/update/cancel` over HTTP or UDS. Use `compozy update --check -o json`,
`compozy update -o json`, and `compozy update --cancel -o json` as the CLI paths.
Read or change the cadence with `compozy config get|set|unset app.update_check -o json` and
`app.update_check_interval`, or the matching `compozy__config_get|set` native tools. Setting
`update_check` to `false` stops background checks on both tracks; manual `--check` remains
available. Both keys apply live without a daemon restart, and an interval change governs the next
background check. Read each key back from the same structured surface after mutation.

## Session Attachments

`session.attachments.max_file_bytes` (10 MiB), `session.attachments.max_files_per_prompt` (10), and
`session.attachments.allowed_mime` (the v1 PNG/JPEG/WebP/PDF/Markdown/plain-text allowlist) control
admission. `session.attachments.retention.max_count` (200),
`session.attachments.retention.max_bytes` (1 GiB), and `session.attachments.retention.max_age` (720h)
control retention. A workspace config overlay supplies the effective values. All six paths are
agent-mutable and restart-required; inspect or change them with `compozy config get|set|unset -o json`
or the typed `compozy__config_*` tools, then confirm the result through a structured read. The v1 store
preserves image bytes, including EXIF metadata; support bundles exclude the attachment tree. Retention
sweeps run at startup, during store access, and every hour while the daemon is running.

## Gateway

`[gateway]` is the operator-global ceiling and tuning section for remote access. It defaults to
`enabled = false` with OS-assigned private and public ports (`0`), pairing TTL `5m` and pending cap
`8`, stream-ticket TTL `30s`, auth failure window `60s` with cap `10`, verification timeout `10s`,
and public DNS-over-TLS verification resolver `1.1.1.1:853`. `gateway.enabled`,
`gateway.verify.public_dns_resolver`, and the bounded duration/count keys apply live; `private_port`
and `public_port` require a daemon restart. Public endpoint proof uses that resolver so a host's
MagicDNS answer cannot substitute the private tailnet route. Configuration can never enable a surface:
`gateway.public_ui.enabled` is invalid, and durable provider/surface intent remains database-owned.

Client-side remote profiles are global metadata under `gateway.active_connection` and
`[[gateway.connections]]`. HTTPS entries require `name`, `scheme = "https"`, `host`, `port`, and
`credential_file = "<name>.cred"`; `default_workspace` is optional. SSH entries use
`scheme = "ssh"`, accept optional `remote_home`, and never reference a Gateway credential. Profile
names contain only letters, digits, hyphens, or underscores and are unique. The active connection
must name an existing profile. Manage these tables with `compozy connect add|list|use|remove|export|import|ssh`
instead of editing them during an active connection lifecycle.

The TOML stores no credential material. Direct HTTPS credentials are encrypted in
`$COMPOZY_HOME/gateway/credentials/<name>.cred`, with the wrapping key held by the operating-system
credential store. Transfer an identity with `compozy connect export <name> --passphrase-file <file>
--output-file <bundle>` and `compozy connect import <bundle> --passphrase-file <file>`; both the
passphrase file and encrypted bundle must be private files. Copying `config.toml` alone does not copy
an identity, and removing a profile removes both its metadata and credential. SSH uses the operator's OpenSSH configuration and agent;
its profile stores only connection metadata.

## Marketplace Catalog

`[marketplace.catalog]` controls CompozyOS's curated MCP server, extension, and skill feed projection.
`base_url` defaults to the public `compozy/compozy` catalog on `main`, `ttl` defaults to `1h`, and
`timeout` defaults to `10s`; all three paths apply live to the next fetch. Use the structured config
surfaces plus `compozy config reload -o json` and apply history to change or verify them. These keys do
not replace the independent `skills.marketplace.*` feed settings or the `extensions.trust.*` and
`extensions.sources.*` distribution settings. `extensions.trust.allow_unverified` applies live; every
other `extensions.*` path is restart-required.

Marketplace catalog configuration is global-only because its projection and refresh service are global. `compozy__config_set` and `compozy__config_unset` may change `marketplace.catalog.ttl` and `marketplace.catalog.timeout` at global scope. `marketplace.catalog.base_url` is a trust root and remains operator-only through global `compozy config set`. Workspace overlays and workspace-scoped writes are rejected.

## Skill Sources

`skills.sources` selects folder conventions besides Compozy's own; the closed preset table is
`agents` (default on) and `claude` (off). Any other name fails with `unknown_skill_source`, which
carries `valid` and `suggestion`. `skills.custom_sources` lists extra directories scanned exactly as
written; absolute and `~/` paths are valid at every scope, workspace-relative paths only at workspace
scope (`invalid_source_path`), and a path another source already owns fails with
`duplicate_skill_source`. Both keys are `live` and pass through the four overlays independently: an
absent key inherits, a list replaces, an empty list disables that key's configured roots. The skills
settings section at workspace scope writes only these two keys; another field returns
`workspace_scope_field_forbidden`.

Read effective sources with `compozy skill sources -o json` or `GET /api/settings/skills`. Both keys
are trust roots, so `compozy__config_set` and `compozy__config_unset` deny them with
`config_trust_root_forbidden`; operators change them with
`compozy config set|unset skills.sources|skills.custom_sources` at user, exact profile
(`--profile <name> --scope profile`), or workspace scope. `PATCH /api/settings/skills` uses the same
profile lens; its workspace body is `{"override": {...}}`, where an absent field is untouched and
`null` clears the override. Workspace-profile config remains repository-authored and read-only.
Before acting on what a source change does to the catalog — precedence, origins, suppression,
exposure — read the Skill sources and exposure section of `references/tools-and-skills.md`.

## Autonomy Scheduler

`[worktrees]` controls the parent-workspace Worktree lifecycle. `root` defaults to
`$COMPOZY_HOME/worktrees`; `run_branch_namespace` defaults to `run/`; `copy_list` and
`setup_command` default empty; `setup_timeout` defaults to `10m`; and `discovery_cache_ttl` defaults
to `30s`. Root must be empty or absolute, the namespace must be lowercase and slash-terminated,
copy entries must be non-empty relative Git pathspecs, and both durations must be positive. The copy
candidate set is limited to ignored, untracked files matched by those pathspecs. Workspace overlays
merge each field independently. `worktrees.*` applies live to later creation and discovery without
moving existing paths or changing an accepted creation.

`task.orchestration.profile.default_worktree_mode` resolves an inherited task worktree policy before
enqueue and accepts `inherit`, `none`, or `per_run`. It defaults to `inherit`, which has root
semantics when no higher policy supplies a mode. The key applies live to later enqueues; existing run
snapshots do not change.

`[autonomy.scheduler]` tunes the mechanical scheduler's convergence escalation ladder for starved runs. Keys are wake-cycle counts that must stay positive and monotonic (`fan_out_after` ≤ `spawn_after` ≤ `event_after` ≤ `needs_attention_after`) plus a `min_queued_age` duration. Defaults: `fan_out_after = 2`, `spawn_after = 4`, `event_after = 6`, `needs_attention_after = 10`, `min_queued_age = "2m"`. Validation rejects non-monotonic or non-positive values at load.

These thresholds apply only to true convergence episodes. Compatible sessions that are starting, prompting, processing another run, or reserved earlier in the scheduler cycle hold serial backlog without consuming the ladder. Policy remains serial: saturation does not start extra task-role capacity.

## Loop Defaults And Observability

`loops.reconcile_interval` defaults to `1m`, accepts only positive durations, and is
restart-required. The interval controls the daemon sweep that settles execution records owned by a
terminal or missing Loop run. Startup performs the same repair before task recovery even when the
next interval has not elapsed. Read or change it through the structured config surfaces and confirm
the stored value before restarting the daemon.

`[loops.defaults.delivery]` and `[loops.defaults.watch]` seed new loop effective config before per-loop `loop_config` overrides; they are desired-state defaults, not the DB-backed override plane. Delivery defaults are `iteration_cap = 50`, `no_progress.window = 3`, `gates.max_revisions = 10`, `budget.tokens = 0`, `budget.wall_clock_sec = 0`, `budget.on_exceeded = "halt"`, and `fan_out_width = 4`. Watch defaults are `iteration_cap = 0`, `no_progress.window = 2`, `budget.tokens = 0`, `budget.wall_clock_sec = 0`, `budget.on_exceeded = "halt"`, and `fan_out_width = 2`; gate revisions remain unset for watch unless configured. Both families accept optional positive `requests.expire_after`, used when an ask or review request omits authored expiry, plus field-merged `runtime_defaults.worker|judge.{provider,model,reasoning}` and ordered `runtime_rules`. Rules match one exact `id`, one `type`, one `complexity`, or the conjunction `type + complexity`. The conjunction is AND. Specificity is `id > type + complexity > type > complexity`; later equal-specificity rules win per non-empty runtime field. Operator config may set any nonnegative `fan_out_width`; it controls the active lane window while each node's positive `max_fan_out` bounds logical width. No-progress window must not exceed `30`, and gate revisions must not exceed `64`. `budget.on_exceeded` accepts only `halt` or `escalate`. These paths are restart-required config lifecycle entries; use `compozy config reload -o json` and apply history to inspect activation.

For example, this matrix rule applies only when both task fields match:

```toml
[[loops.defaults.delivery.runtime_rules]]
[loops.defaults.delivery.runtime_rules.match]
type = "frontend"
complexity = "high"

[loops.defaults.delivery.runtime_rules.runtime]
provider = "claude"
model = "opus"
reasoning = "high"
```

Declared Loop inputs may have global or workspace defaults under `[loops.inputs.<loop-name>]`.
Scalar values and partial runtime objects are writable through
`compozy config get|set|unset loops.inputs.<loop-name>.<key>` and the matching native config tools.
Entity existence is checked when the effective value is used, not when config is written. Resolution
order and the rest of the lifecycle live in `references/loops.md`.

Loop observability is durable runtime state, not a transient UI stream. `loop_run_events` persists replayable workspace-scoped events for status changes, node running/terminal outcomes, gate verdicts, generation starts, channel messages, token ticks, and needs-approval pauses. Payloads are redacted and bounded before persistence; token ticks preserve only usage counters and terminal markers.

## Goals

`[goals]` sets `max_turns = 20` and `context_nudge_ratio = 0.8` for new Goals, plus the daemon-wide durable session-event relay controls `outbox_batch_size = 50` and `outbox_poll_interval = "100ms"`. The Goal defaults are global/workspace-overridable; relay controls use global config because one relay serves every workspace. All four are agent-mutable, restart-required paths. `max_turns` must be positive; the ratio accepts `0.0` through `1.0`, with zero preserved; the relay batch accepts `1` through `200`; and its poll interval must be positive. Each Run pins its resolved ratio and every Goal checkpoint copies that value, so config reload or daemon restart cannot change an active Goal. Relay settings take effect when the daemon starts.

## Profile Selection Environment

Profiles add no `config.toml` key. `COMPOZY_PROFILE` selects one active profile for commands in the
current process environment, after an explicit root `--profile` flag and before the workspace's
remembered choice. `daemon`, `doctor`, and `update` ignore both inputs. Persist a workspace or Global
lens choice with `compozy profile use`; read `references/profiles.md` for the complete precedence and
lifecycle contract.

## Automation Schedules

Automation schedule catch-up policy is part of the public schedule contract. Recurring schedules accept `skip_missed`, `coalesce`, `replay`, and `run_once_on_catchup`; one-time `at` schedules reject catch-up fields. Omit the policy for the target-aware default: Loop targets with a `watch-source` use `coalesce`, while other scheduled targets use `skip_missed`. `misfire_grace_seconds=0` uses the daemon jitter grace. Durable canceled runs identify `misfire_grace_exceeded` and `self_overlap` under `metadata.reason`. Catch-up starts carry structured automation-run metadata so agents can distinguish normal starts from recovered starts and reason about `concurrency: forbid|queue` outcomes.

CLI automation creation supports the Agent-or-Loop target union. Use `automation jobs create --loop` with repeatable `--loop-input` values for scheduled Loop starts; triggers also accept repeatable `--loop-input-mapping` templates. Global definitions require `--loop-workspace`. CLI updates change common fields without replacing the target; use native tools or HTTP/UDS for target replacement.

Trigger events are `session.created`, `session.stopped`, `memory.consolidated`, `hook.<hook_name>.completed`, `webhook`, or `ext.*`. Unknown names and surrounding whitespace are rejected; `ext.*` suffixes stay free-form.

## Session Compaction

`[session.compaction]` controls pressure-triggered checkpoint coverage and replay archiving. Defaults
are `enabled = true`, `pressure_threshold = 0.85`, `max_attempts_per_turn = 1`, and
`failure_cooldown = "10m"`; threshold zero disables admission. All paths are available through
`compozy config set` and the native config tools, are restart-required under the canonical `session.*`
lifecycle rule, and do not mutate the policy bound to the running daemon.

## Auto-Title Role

`roles.auto_title.enabled` defaults to `true` and gates the daemon-owned title pass for unnamed user
sessions after their first persisted assistant response. The remaining `roles.auto_title.*` fields
select its agent, provider, model, reasoning effort, and ordered fallback routes. Role changes are
Live desired state for later invocations at global or workspace scope. Explicit names win; disabled
or failed generation leaves the session unnamed.

Other `[roles]` routing keys and the fallback-chain rules live in `references/runtime-operations.md` (Background roles).

## Persona Defaults

Persona defaults select the agent, provider, and sandbox used for new work. Read them through
`GET /api/settings/persona` and update them through `PATCH /api/settings/persona`. Omit `scope` for
the user layer; use `scope=profile&profile=<name>` for a personal profile layer. The response names
the effective scope, profile owner, available scopes, write target, and the `agent`, `provider`, and
`sandbox` defaults. Profile updates affect only later work started under that profile.

## Command Palette

`[cmd_palette]` owns `fallback_targets`, the live `personalization` flag, and the
`[cmd_palette.aliases]` command-to-alias map. `fallback_targets = ["agent"]` enables the agent row for
a non-empty query without a strong result; `[]` disables it. Enter creates a new default-agent
session with the query as its opening prompt, and no query is sent before Enter. Aliases are
workspace-scoped vocabulary: 1–32 characters, no whitespace, and unique per workspace. Read or
write scalar settings with `compozy config get|set cmd_palette.<key>`. Use the typed
`GET|PATCH /api/settings/cmd-palette` surface for live fallback and personalization controls, and
`compozy cmd-palette alias set|clear` for atomic alias changes through the daemon.

## Window Manager

`[window_manager]` controls global behavior defaults for new-window placement, small-viewport
fallback, focus and raise policy, drag-away grouping, bounded history, desktop transitions, gaps,
snap thresholds and repeat ratios, edge bindings, and shortcuts. Every `window_manager.*` path is
live-applied only after the complete candidate validates; a failed apply keeps the prior active
generation. Workspace topology overrides remain part of revisioned layout documents rather than a
second Settings scope. Field detail lives in `references/window-management.md` (Configuration and hooks).

Shortcut entries accept a string, a string array, an empty binding, or an indexed range for
`desktop.switch` (`1..9`) and `window.tab.jump` (`1..8`). Read
`compozy config get window_manager -o json` to inspect daemon defaults and the effective map before
writing an override. The daemon expands
ranges and validates the full map atomically; a collision stores nothing.

Use `compozy cmd-palette bindings` to read the workspace effective keymap, aliases, dormant
extension defaults, and conflicts. `bind|unbind` mutate workspace bindings; `--overwrite` transfers
a conflicting chord from the owner named by the daemon.

`[window_manager.global_shortcuts]` maps any registry command ID to one desktop-global chord. It
defaults `palette.summon.global = "meta+shift+Space"`. Use `compozy cmd-palette bind <id> <chord>
--global` or `unbind <id> --global`; `--overwrite` transfers a collision atomically. The daemon owns
intended bindings, while each Electron shell reports its own `registered`, `failed_in_use`,
`failed_permission`, or `unsupported` state. A plain browser cannot register these chords.

## Attention

`[attention]` controls operator notification delivery. `toasts` and `sound` default to `true`,
and `system` defaults to `false`. Every `attention.*` config path applies live as one validated
candidate.

Workspace mutes are profile-owned state, not `config.toml` keys. Read or replace the default
profile's mute set with `GET/PATCH /api/settings/attention?scope=user`; use
`scope=profile&profile=<name>` for another profile. A muted workspace receives no notification
event, but its attention rows and counts remain unchanged. Workspace removal deletes every profile's
mute row through the workspace foreign key.

Use `compozy config get|set attention.toasts|sound|system` for global delivery controls, or the typed
Settings route above for the complete view. The title count is always on and is not a config key.

## Shell Session Preferences

`shell.sessions.sort` persists the session list order and accepts `last_activity` (default) or
`attention`. `shell.sessions.scope` persists the list breadth and accepts `workspace` (default) or
`all-workspaces`. Both keys are global, apply live, and survive browser restarts in
`$COMPOZY_HOME/config.toml`; workspace-scoped writes are rejected, and browser-local storage is not
authoritative.

Use `compozy config get|set shell.sessions.<key>` or `GET/PATCH /api/settings/shell`. The PATCH
request replaces the complete `shell` config section, so read the current section before changing
one field.
