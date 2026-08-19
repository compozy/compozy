# Native Tools

## Contents

- Operating rule
- Discovery and catalog toolsets
- Command palette tools
- Runtime and workspace tools
- Workspace boundary
- Window management tools
- Skills and memory tools
- Network tools
- Task and autonomy tools
- Loop tools
- Config, hooks, automation, marketplace, extensions, resources, and MCP tools
- Observability and bridge tools
- CLI/HTTP-only management surfaces
- Descriptor discipline

## Operating Rule

Inside CompozyOS, prefer callable daemon-native tools over shelling out. They are policy-filtered, structured, auditable, and redaction-aware. Use shell only when a native tool is absent, denied, too narrow, or explicitly requested.

Never guess a tool schema from this reference. Resolve canonical `compozy__tool_info` for the exact descriptor, input schema, risks, and availability diagnostics before the first call.

Management-only surfaces include diagnostics, support bundles, scheduler controls, task inspection/pause/force recovery, notification presets, config apply history, and some session repair/recap/approval flows.

`workspace` is optional and defaults to the bound session's workspace. Naming another workspace is a cross-workspace request governed by the session permission mode; see Workspace Boundary below. Bound sessions still cannot use `global`/`all`; operators can.

## Discovery And Catalog Toolsets

- Toolset `compozy__bootstrap`: `compozy__tool_list`, `compozy__tool_search`, `compozy__tool_info`.
- Toolset `compozy__catalog`: agent, command, skill, and redacted Vault-reference catalog access plus bootstrap tools.

Bare managed sessions receive the full availability-gated callable catalog through hosted MCP. CompozyOS
does not add an automatic bootstrap/catalog allowlist over the hosted projection. Explicit agent
`tools`, `toolsets`, `deny_tools`, session lineage, disabled sources, and approval/risk gates still
apply.

The discovery loop and denial-handling rules live in the preceding tools-and-skills prompt section; this reference supplies the stable tool map.

## Command Palette Tools

Use `compozy__cmd_palette_list` to read the daemon-canonical command catalog for the bound workspace.
Optional `source` filters one provider and optional `client` resolves client-context availability.

Use `compozy__cmd_palette_invoke` with `id`, optional `args`, and optional `client`. The command's own
availability, targeting, single-flight, and approval rules still apply. An `approval_pending` result
returns `approval_id`; operators inspect or cancel it with `compozy approvals show|cancel <id>`.
CLI catalog fallback is `compozy cmd-palette list|inspect|invoke|clients`.

## Runtime And Workspace Tools

Session tools: `compozy__session_list`, `compozy__session_create`, `compozy__session_prompt`,
`compozy__session_rewind`,
`compozy__session_status`, `compozy__session_history`, `compozy__session_events`,
`compozy__session_describe`, `compozy__session_health`, `compozy__session_runtime_set`,
`compozy__session_runtime_clear`, `compozy__session_archive`,
`compozy__session_unarchive`, `compozy__session_rename`, `compozy__session_wait`,
`compozy__session_spawn`, `compozy__session_stop`, `compozy__session_approve`,
`compozy__session_clarify_answer`, `compozy__session_prompt_cancel`.

`compozy__notify` sends one operator notification from the bound session and workspace. Pass a
required `title` of at most 80 characters and an optional `body` of at most 240 characters. The
daemon sanitizes and redacts both fields before delivery, then returns exactly one provable outcome:
`delivered` when at least one live operator catalog-stream subscriber accepted the event,
`no-client`, `muted-workspace`, or `rate-limited` with `retry_after_ms`. The per-session limit is one
send per second, including sends suppressed by a workspace mute. CLI fallback: `compozy notify`.

`compozy__session_create` accepts workspace, agent, optional name, and exactly one of `worktree` or
`new_worktree`. It creates an active logical session with `runtime.status="unbound"`; it does not
accept or send a prompt or runtime. A named Worktree must be ready; new creation reaches ready before
session persistence.
The calling session is recorded automatically as `lineage.parent_session_id` (provenance only, no
governance) when the new session lands in the caller's workspace; the link is not a tool input.
Use `compozy__session_prompt` with `session_id`, optional `message`, optional `attachments`, and an
optional `runtime` snapshot. `attachments` is an array of file paths within the resolved workspace
root or configured `additional_dirs`, or existing workspace/session-scoped `att_...` IDs; at least
`message` or one attachment is required. Paths are
uploaded through the attachment store before submission. Use
`compozy session attachments upload <session-id> <file> -o json` when the caller needs a durable ID.
The store enforces `session.attachments.max_file_bytes`,
`session.attachments.max_files_per_prompt`, and `session.attachments.allowed_mime`; inspect the
attachment settings in `references/configuration.md` before submitting large or unsupported files.
The first prompt to an unbound session requires `runtime.provider`; model, reasoning effort, and speed
are optional snapshot fields. Read the runtime semantics, queued/interrupt snapshot behavior, and
rollback rule in `references/runtime-operations.md`; inspect `compozy__session_status` for the nested
`runtime` object rather than deriving it from the session state.

`compozy__session_wait` blocks on one same-workspace session other than the caller. `until` accepts
the canonical attention/lifecycle badges; omission uses the settled set, and `done` satisfies `idle`.
`timeout_ms` defaults to 300,000 and cannot exceed 1,800,000. Outcomes are `state-reached`, `timeout`,
`session-gone`, `canceled`, and `overflow`. A native wait emits activity heartbeats while blocked so
session supervision does not mistake orchestration for inactivity.

`compozy__session_spawn` creates one governed child of the caller. It requires `agent_name` and a
positive `ttl_seconds`; optional permission arrays can only narrow the parent. Omitted
`notify_creator` defaults to true, while explicit false suppresses the synthetic parent wake for
that child. The result returns the child session ID, role, depth, and TTL expiry.

`compozy__session_stop` stops another same-workspace session and is destructive/approval-gated. It is
idempotent for an already stopped target. `compozy__session_prompt_cancel` cancels only another
session's active prompt and returns `canceled` or `nothing-in-flight` without stopping the session.

`compozy__session_approve` resolves another session's permission request with `allow-once`,
`allow-always`, `reject-once`, or `reject-always`. `compozy__session_clarify_answer` resolves another
session's pending question with either a one-based `choice` or text. Both preserve durable
post-restart interactions and can return `already-resolved`, `resolved-after-restart`, or
`queue-full`; self-action is denied. Read `compozy session interactions` or
`compozy__session_status` for the durable request IDs instead of matching display text.

`compozy__session_rewind` is a destructive conversation-only mutation. Pass the durable user
`message_id`, an idempotency key, and the transcript epoch, generation, and maximum sequence returned
by the read API. The tool cuts before that message, restarts a fresh ACP context under the same
CompozyOS session ID, and returns the selected text as `draft_text`. It never rolls back files, tool or
network effects, memory, or external provider actions. Resolve its descriptor and obtain approval
before calling it.

`compozy__session_runtime_set` persists complete next-prompt intent without starting or
reconfiguring ACP; `compozy__session_runtime_clear` removes it. Both accept optional
`expected_revision`; when omitted, the tool reads the current `runtime.selection_revision` before
the write. On conflict, read session status and decide from the new server value instead of replaying
the stale mutation.

Worktree tools: `compozy__worktree_list`, `compozy__worktree_inspect`,
`compozy__worktree_create`, and `compozy__worktree_remove`. List and inspect are read tools; create is
mutating; remove is destructive and approval-gated. All carry an explicit or caller-resolved
workspace boundary. Adopt, creation cancel, exit planning/actions, exit cancel, and dismiss are
CLI/HTTP/UDS-only; read `references/worktrees.md` before using either path.

Remembered approvals: `compozy__tool_approvals_set`, `compozy__tool_approvals_list`, and
`compozy__tool_approvals_revoke`. `allow-always` or `reject-always` creates an exact workspace + agent +
tool + input-digest decision. Explicit set accepts only `agent` or `tool` scope and no input digest;
an agent-wide set requires `agent_name`. Wider allows remain below the configured tool-policy
ceiling. CLI: `compozy tool approvals set|list|revoke --workspace <workspace>`.

Clarification: `compozy__clarify` asks one active-session question with at most four choices and returns
zero-based `{choice,text,fallback}`. It is not approval. CLI: `compozy session clarify pending|answer`
(choice presentation is one-based).

`compozy__session_list` returns one counted catalog page and accepts workspace, exact state, exact
session `type`, exact agent, exact `parent` (direct children) or `root` (whole tree, root included),
search, resumability, health, archive visibility, sort, cursor, and limit inputs. Archive visibility defaults to `exclude`; use `only` or `include` when the workflow needs
archived rows. Use `type: "user"` when a workflow needs operator-created sessions without
daemon-managed dream, system, coordinator, or spawned sessions. Archive only stopped sessions;
unarchive before prompting or resuming one. Archive and conversation clear preserve attachments;
conversation clear preserves attachments. `compozy session remove` and `compozy workspace remove`
remove their scoped attachment trees.
`compozy__session_rename` changes the durable display name of a user session without changing its
runtime or transcript identity. Pass `session_id` and a non-empty `name` of at most 64 characters;
the CLI fallback is `compozy session rename <id> <name>`.

Authored context tools: `compozy__agent_heartbeat_status`, `compozy__agent_heartbeat_wake`.

Workspace tools: `compozy__workspace_list`, `compozy__workspace_info`, `compozy__workspace_describe`,
`compozy__agent_list`, `compozy__agent_create`. `compozy__agent_list` returns exact names from the
resolved workspace catalog. `compozy__agent_create` authors one public `AGENT.md` at `global` or
`workspace` scope; provide `scope`, `name`, `prompt`, and `workspace` for workspace scope. Provider,
model, and reasoning are optional agent-level overrides; when omitted, the definition inherits the
target project runtime defaults.

Fresh daemon boot registers the operator `$HOME` as the default workspace through the resolver, so `compozy__workspace_list` should return at least that workspace on a clean install.

A successful workspace catalog read reconciles registered roots before returning: entries whose directories no longer exist are durably unregistered, while other filesystem or deletion failures fail the read instead of hiding uncertain state. `compozy__workspace_list`, `compozy workspace list`, and HTTP/UDS `GET /api/workspaces` share this catalog.

The workspace setup browser reads `GET /api/fs/browse`; its response includes `home` and every
filesystem `root` so the UI can navigate outside the operator home. Agents should register a known
path through the workspace management surface instead of browsing interactively.

Workspace unregister is atomic with credential cleanup: it removes workspace-scoped MCP OAuth rows and their encrypted access/refresh values, preserves global and sibling-workspace credentials, and leaves all state intact when cleanup fails.

Provider model tools: `compozy__provider_models_list`, `compozy__provider_models_curate`, `compozy__provider_models_refresh`, `compozy__provider_models_status`.

Vault catalog tool: `compozy__vault_list`. It returns global reference names and redacted metadata,
optionally filtered by prefix. It never returns secret values.

Gateway tool: `compozy__gateway`, in toolset `compozy__gateway`. Resolve its live descriptor before
calling it. `status`, `audit`, and `device_list` are read actions and remain callable in every
session permission mode while the Gateway service is available. Management actions are
`surface_set`, `provider_enable`, `provider_disable`, `device_rename`, `device_revoke`,
`ingress_bind`, and `ingress_unbind`. A local operator may call them directly; an agent session must
have effective permission mode `approve-all`. Lower modes return `tool_denied` with
`policy_denied`. The tool returns only redacted projections and intentionally has no pairing,
credential, or stream-ticket issuance action. Use the local CLI or private management API when a
one-time secret must be issued.

`compozy__provider_models_list` accepts `view=curated|all` and defaults to curated; the CLI equivalents are `compozy provider models list` and `compozy provider models list --all`. `compozy__provider_models_curate` is mutating, requires `providers.models.write`, and accepts required `provider_id`/`model_id` plus optional `hidden`, `featured`, `deprecated`, and `default_effort`. Its CLI fallback is `compozy provider models set`. Treat `model_not_found` and `reasoning_effort_unsupported` as terminal input diagnostics; when the descriptor reports the settings backend unavailable, do not retry blindly.
For providers with an explicit curated set, the default view contains visible explicit or featured rows; live-only rows appear there only through the no-explicit-set fallback.
Cursor catalog discovery starts a short-lived `cursor-agent acp` session and stores the exact values
advertised by its `model` option under `provider_live:cursor`. Do not use the human-readable
`cursor-agent models` aliases as model IDs. An explicit Cursor model refreshes and must match the
live catalog; omitting it preserves Cursor's native default. Refresh through
`compozy__provider_models_refresh` or `compozy provider models refresh cursor`.
`providers.cursor.models.discovery.*` config writes apply live: changed discovery wiring refreshes the Cursor source, while model metadata-only writes do not invoke the provider.
Model-list and curation results may include a `cost` object with independent `input_per_million`, `output_per_million`, `cache_read_per_million`, `cache_write_per_million`, and `reasoning_per_million` fields. A missing field means that bucket is unpriced; never infer it from another field.

Provider authentication is a management surface. Write `providers.<id>.auth_login_command` only through `config.toml`, `compozy config set`, or `compozy__config_set`; it is write-only and redacted from config show, list, get, diff, and set reads. Provider status, doctor, API/UDS, Settings, and Web expose for it only `{configured, source, executable, presence, recommended_action}`, where `executable` is the basename. `compozy provider auth login <provider>` executes the configured login command internally and never returns the raw command.

## Workspace Boundary

A call that names a workspace other than the bound session's is a cross-workspace request. The session's effective permission mode decides it, and there is no separate toggle, grant, or config key:

- `approve-all`: allowed at every seam.
- `deny-all`: denied at every seam, including native tools. CompozyOS never prompts for crossing under this mode.
- `approve-reads`: the native-tool seam prompts the operator while the session is running; the agent-identity, task, spawn, and workspace-coordination seams deny.

Denied native calls return `workspace_access_denied` with this exact hint:

    cross-workspace access is denied by this session's permission mode; ask the operator to set the agent's permissions.mode to approve-all, or approve the prompt when asked

The prompt offers four daemon-computed options: `allow_once` (this call), `allow_session` (this call and every later crossing by this session, at every seam), `reject_once` (this call), and `reject_session` (this call and every later crossing by this session). A `*_session` answer lives only in daemon memory, expires when the session stops or the daemon restarts, and has no list or revoke surface. A prompt timeout, transport failure, or unknown answer denies and stores no session answer.

Each policy evaluation attempts a best-effort `workspace.access_granted` or `workspace.access_denied` audit append with target workspace, seam, decision source, and mode. A prompt-eligible miss is a denied policy evaluation even when the operator then allows that one call; later evaluations allowed by `allow_session` are granted. Read persisted events through `compozy__logs`, `compozy__observe_search`, `compozy logs --type <event-type>`, or `GET /api/logs`.

On denial, report the hint to the operator and stop that line of work. Do not retry the same call, re-enter through another seam, spawn a child to cross for you, or edit `permissions.*` in config or an agent definition. `compozy__config_set` on `permissions.*` fail-closes; the operator owns the mode.

## Window Management Tools

Toolset `compozy__window_manager` requires `window_manager.read` for reads and `window_manager.write`
for mutations. Resolve descriptors, pass the workspace, and use the current revision.

- Desktops: `compozy__desktop_list`, `compozy__desktop_create`, `compozy__desktop_update`,
  `compozy__desktop_reorder`, `compozy__desktop_switch`, `compozy__desktop_delete`,
  `compozy__desktop_clients`.
- Windows: `compozy__window_list`, `compozy__window_open`, `compozy__window_close`, `compozy__window_focus`,
  `compozy__window_move`, `compozy__window_swap`, `compozy__window_float`, `compozy__window_zoom`,
  `compozy__window_navigate`.
- Tabs: `compozy__window_group`, `compozy__window_reorder`, `compozy__window_activate`,
  `compozy__window_pin`, `compozy__window_reopen`.
- Layouts: `compozy__layout_get`, `compozy__layout_preview`, `compozy__layout_arrange`,
  `compozy__layout_resize`, `compozy__layout_balance`, `compozy__layout_undo`, `compozy__layout_redo`,
  `compozy__layout_export`, `compozy__layout_validate`, `compozy__layout_apply`.

Every tab tool mutates and needs `window_manager.write`. `window_group` takes `target_window_id` plus
a `window_ids` array and an optional `insert_index`; `window_reorder` moves one member inside its own
stack with a clamped `index`; `window_activate` sets the stack's durable active member and is the
public name of the internal `window.stack.set_active` command; `window_pin` takes a required `pinned`
boolean; `window_reopen` needs only the revision. Three existing tools carry tab inputs:
`window_open` accepts `stack_target_window_id`, `window_navigate` accepts `mode`
(`replace`/`push`/`pop`, and `pop` forbids `route`), and `window_close` accepts `scope`
(`tab`/`group`/`others`/`right`, rejected together with `minimize`).

`desktop_switch`, `window_focus`, and `window_zoom` require a connected `client_id`.
`window_navigate` changes presentation only when given one. Preview/validate never write;
desktop delete, window close, and layout apply carry destructive risk. Discover `window_layout`
resources with `compozy__resources_list`; `resource_id` is exclusive with inline arrange fields. CLI
fallbacks are `compozy desktop|window|layout`; read `window-management.md` for multi-step changes.

## Skills And Memory Tools

Command catalog tool: `compozy__command_list`. Pass `session_id`; optional `workspace` defaults to the
bound session workspace. The result is the daemon-owned command projection for that session.

Skill tools: `compozy__skill_list`, `compozy__skill_search`, `compozy__skill_view`. Use `name` for a
normal registry read. Use exactly one source-qualified `command_id` from `compozy__command_list` when
the operator selected a slash skill; `command_id` requires a session-bound caller and never switches
to another skill source.

Resolve canonical `compozy__skill_view`, then use its returned tool reference with a file/resource argument when reading `skills/compozy/references/*.md` from inside CompozyOS.

Memory tools: `compozy__memory_list`, `compozy__memory_show`, `compozy__memory_search`, `compozy__memory_propose`, `compozy__memory_note`.

Memory admin tools include health, scope, reindex, promote, reset, reload, decisions, recall traces, dreams, daily logs, extractor, provider, and session-ledger operations under the `compozy__memory_*` namespace. Inspect descriptors before using admin tools because they are broader than normal memory reads.

## Network Tools

Coordination tools: `compozy__network_status`, `compozy__network_channels`, `compozy__network_channel_create`, `compozy__network_channel_update`, `compozy__network_inbox`, `compozy__network_peers`, `compozy__network_send`, `compozy__network_threads`, `compozy__network_thread_messages`, `compozy__task_promote_from_thread`, `compozy__network_subscriptions`, `compozy__network_subscribe`, `compozy__network_mute`, `compozy__network_unmute`, `compozy__network_directs`, `compozy__network_direct_resolve`, `compozy__network_direct_messages`, `compozy__network_work`.

Channel create/update are mutating. Channel names are lowercase `[a-z0-9][a-z0-9_-]{0,63}`;
coordinator routing metadata requires `coordinator_peer_id`. Routing metadata never enrolls or wakes
an execution.

The coordination toolset is projected only when the caller's immutable participation snapshot is
Live, then narrowed by policy and dependency gates. Daemon availability alone never exposes it. A
Local caller receives `not_participating`; create a new explicitly Live execution instead of
retrying.

Read references/network.md before sending or interpreting messages. Direct/mention `say` is the
only current model-wake path; other messages may persist without activation. Use `compozy network usage
-o json` through CLI/HTTP/UDS for usage because it is a management read, not a native coordination
tool ID.

## Task And Autonomy Tools

Task tools: `compozy__task_list`, `compozy__task_read`, `compozy__task_create`, `compozy__task_child_create`, `compozy__task_update`, `compozy__task_cancel`, `compozy__task_promote_from_thread`, `compozy__task_fanout_runs`, `compozy__task_run_list`, `compozy__task_run_review_request`, `compozy__task_run_review_list`, `compozy__task_run_review_show`, `compozy__task_execution_profile_get`, `compozy__task_execution_profile_set`, `compozy__task_execution_profile_delete`, `compozy__task_worktree_policy_set`, `compozy__task_notification_subscribe`, `compozy__task_notification_list`, `compozy__task_notification_show`, `compozy__task_notification_delete`.

Task-notification cursor diagnostics expose an explicit `{kind, workspace_id}` scope, with `kind`
closed to `global` or `workspace`. Subscribe and list use the hard-cut `workspace_id` input; do not
send the removed `workspace` field. Preserve task, subscription, bridge, workspace, peer, group,
thread, and delivery IDs byte for byte as non-empty valid UTF-8 where required. Never trim,
case-fold, prefix, split, or reconstruct them. A bridge terminal cursor's `consumer_id` is exactly
its `subscription_id`, and a bridge acknowledgment must echo the exact `delivery_id`.

Session-bound autonomy tools: `compozy__task_run_claim_next`, `compozy__task_run_heartbeat`, `compozy__task_run_complete`, `compozy__task_run_fail`, `compozy__task_run_release`, `compozy__task_run_review_submit`.

Autonomy tools are bound to the caller session. `compozy__task_run_claim_next` claims and starts a worker run; a different returned `session_id` means execution moved to that dedicated session and the caller must end its turn. Do not substitute general task mutation tools for session-bound lease operations. Read references/tasks-and-orchestration.md before claiming, heartbeating, completing, failing, releasing, or submitting review verdicts.

## Loop Tools

Toolset `compozy__loops` has 23 tools: Goal get/report; Loop
list/inspect/validate/create/run/status/runs/turns/cancel/kill/pause/resume/configure/approve/delete;
and node list/pause/resume/cancel/kill/requeue.

No `compozy__loop_edit`. See references/loops.md for publishing, approval/self-approval, and Goal report
binding semantics.

## Config, Hooks, Automation, Marketplace, Extensions, Resources, And MCP Tools

Config tools live under `compozy__config_*` for show/list/get/set/unset/diff/path. Hook tools live under `compozy__hooks_*` for list/info/events/runs/create/update/delete/enable/disable; hooks are typed dispatch, not an event bus.

Background-role inspection has no `compozy__roles_*` native tool. Use `compozy roles list|show -o json` or
the HTTP/UDS `GET /api/roles` reads. Scalar `roles.<role>.*` routing and role-policy keys are exposed
through the live `compozy__config_set`/`compozy__config_unset` descriptors, including coordinator limits and
memory-controller call bounds. Fallback chains are structured arrays and must be changed through
`config.toml` or the Settings Roles API/UI, not guessed into a scalar config-tool call. Inspect the
live descriptor before any mutation; successful role writes report the `live` lifecycle and affect
later invocations.

Automation catalogs use CLI, HTTP/UDS, and `compozy__automation_jobs_list` / `compozy__automation_triggers_list`.
Their counted cursor pages filter by scope/workspace, source, enabled, Loop target, search, and event;
run history stays uncounted and must be bounded. Other `compozy__automation_*` tools cover detail,
mutation, toggles, and manual trigger. Config/package definitions only toggle enabled and cannot be
deleted; dynamic definitions are fully mutable.

`compozy__automation_suggestions_{list,accept,dismiss}` accepts optional `workspace`; list, accept, or
dismiss; retry CAS conflicts.

`compozy__marketplace_search` returns MCP, extension, and skill rows. Single-kind cursors bind the
query, scope, workspace, and source projection; grouped searches omit them. Paging and installed
identity rules live in `references/tools-and-skills.md`. Installed state is scoped to the caller's
exact workspace; never reuse it across workspace scopes.
Extension tools are
`compozy__extensions_{init,build,validate,dev,reload,logs,list,info,inventory,preview,search,provenance,publish,install,update,remove,enable,disable}`.
`validate`, `logs`, `list`, `info`, `search`, and `provenance` are read-only; `init`, `build`, `dev`,
and `reload` are mutating, with `build`, `dev`, and `reload` requiring interaction; `remove` is
destructive; `publish` is open-world, interaction-gated, and takes no credential field because the
daemon resolves and redacts that token server-side. `compozy__extensions_search` spans the `curated`
and `github` sources only and is not `compozy__marketplace_search`. Resolve live schemas first;
install sources, consent, authoring, workspace binding, generation handles, commit boundaries,
batch-update outcomes, kit lifecycle, and cleanup warnings live in `references/extensions.md`.

The `compozy__automation_jobs_create` and `compozy__automation_jobs_update` descriptors expose the complete recurring schedule shape, including `catch_up_policy` and `misfire_grace_seconds`. Resolve the live descriptor instead of guessing the enum or sending catch-up fields to a one-time `at` schedule.

`compozy__automation_jobs_create` and `compozy__automation_jobs_update` reject Agent prompts and Task descriptions containing command-shaped CompozyOS daemon restart, stop, or kill instructions before persistence, including resource-applied definitions. The tool error names `compozy_daemon`, `process_signal`, or `service_manager`; remove the lifecycle command before retrying. There is no bypass.

Resource tools live under `compozy__resources_*` for list/info/snapshot of desired-state resources.

MCP diagnostics are `compozy__mcp_status` and `compozy__mcp_auth_status`. A
workspace-scoped server with five consecutive confirmed permanent failures reports `state: "dead"`
from `compozy__mcp_status`; its nested runtime reason is `backend_dead`. During the same daemon lifetime,
resolve its last-known tools through `compozy__tool_info`, which retains their unavailable descriptors
and diagnostic instead of hiding them. Do not retry a dead tool blindly or invent a revive call: CompozyOS
admits at most one automatic recovery probe after the 60-second window and clears the mark when that
probe succeeds. Browser/OAuth login, raw auth material, and any required credential repair remain
management-surface operations.
Curated install remains a management surface (`compozy mcp install` or
`POST /api/settings/mcp-servers/install`); there is no `compozy__mcp_install`.

## Observability And Bridge Tools

Runtime log inspection is available through `compozy__logs`. Metrics and redacted event search are available through `compozy__observe_metrics` and `compozy__observe_search`.

Bridge list/status uses CLI, HTTP/UDS, and `compozy__bridges_list` / `compozy__bridges_status` for counted,
filtered, redacted pages. The health stream accepts at most 200 current-page IDs in the same scope.
Lifecycle, routes, secrets, `manifest`, `setup`, `verify`, real `send-test`, and webhooks stay on
CLI/HTTP/UDS unless the live descriptor exposes a scoped native tool.

## CLI/HTTP-Only Management Surfaces

CLI/HTTP/UDS owns diagnostics (`compozy status`, `compozy doctor`), session repair/recap/approval/inspect/soul
refresh, task inspection/control, schedulers, config reload/history, notification presets, and support
bundles. Task notification subscriptions are native; presets are not. Use management surfaces unless
the live registry exposes a matching `compozy__*` descriptor.

## Descriptor Discipline

This reference gives the stable map. The live descriptor gives exact input schema, output shape, risk flags, availability reason codes, and policy/dependency diagnostics.

If a descriptor is unavailable or denied, do not retry blindly. Choose a narrower tool, read-only status path, or CLI/control surface based on the reason code.
