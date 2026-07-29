# Native Tools

## Contents

- Operating rule
- Discovery and catalog toolsets
- Runtime and workspace tools
- Workspace boundary
- Window management tools
- Skills and memory tools
- Network tools
- Task and autonomy tools
- Loop tools
- Config, hooks, automation, marketplace, extensions, bundles, resources, and MCP tools
- Observability and bridge tools
- CLI/HTTP-only management surfaces
- Descriptor discipline
- Descriptor and skill co-ship

## Operating Rule

Inside Compozy, prefer callable daemon-native tools over shelling out. They are policy-filtered, structured, auditable, and redaction-aware. Use shell only when a native tool is absent, denied, too narrow, or explicitly requested.

`compozy__*` strings are canonical ToolIDs for registry, policy, CLI, descriptors, and `tool_id`; harnesses may wrap call names. Resolve by capability plus canonical ID, then call the returned reference exactly.

Never guess a tool schema from this reference. Resolve canonical `compozy__tool_info` for the exact descriptor, input schema, risks, and availability diagnostics before the first call.

Management-only surfaces include diagnostics, support bundles, scheduler controls, task inspection/pause/force recovery, notification presets, config apply history, and some session repair/recap/approval flows.

`workspace` is optional and defaults to the bound session's workspace. Naming another workspace is a cross-workspace request governed by the session permission mode; see Workspace Boundary below. Bound sessions still cannot use `global`/`all`; operators can.

## Discovery And Catalog Toolsets

- Toolset `compozy__bootstrap`: `compozy__tool_list`, `compozy__tool_search`, `compozy__tool_info`.
- Toolset `compozy__catalog`: skill catalog access plus bootstrap tools.

Bare managed sessions receive the full availability-gated callable catalog through hosted MCP. Compozy
does not add an automatic bootstrap/catalog allowlist over the hosted projection. Explicit agent
`tools`, `toolsets`, `deny_tools`, session lineage, disabled sources, and approval/risk gates still
apply.

The discovery loop and denial-handling rules live in the preceding tools-and-skills prompt section; this reference supplies the stable tool map.

## Runtime And Workspace Tools

Session tools: `compozy__session_list`, `compozy__session_status`, `compozy__session_history`, `compozy__session_events`, `compozy__session_describe`, `compozy__session_health`.

Remembered approvals: `compozy__tool_approvals_set`, `compozy__tool_approvals_list`, and
`compozy__tool_approvals_revoke`. `allow-always` or `reject-always` creates an exact workspace + agent +
tool + input-digest decision. Explicit set accepts only `agent` or `tool` scope and no input digest;
an agent-wide set requires `agent_name`. Wider allows remain below the configured tool-policy
ceiling. CLI: `compozy tool approvals set|list|revoke --workspace <workspace>`.

Clarification: `compozy__clarify` asks one active-session question with at most four choices and returns
zero-based `{choice,text,fallback}`. It is not approval. CLI: `compozy session clarify pending|answer`
(choice presentation is one-based).

`compozy__session_list` returns one counted catalog page and accepts workspace, exact state, exact session `type`, exact agent, search, resumability, health, sort, cursor, and limit inputs. Use `type: "user"` when a workflow needs operator-created sessions without daemon-managed dream, system, coordinator, or spawned sessions.

Authored context tools: `compozy__agent_heartbeat_status`, `compozy__agent_heartbeat_wake`.

Workspace tools: `compozy__workspace_list`, `compozy__workspace_info`, `compozy__workspace_describe`, `compozy__agent_create`. `compozy__agent_create` authors one public `AGENT.md` at `global` or `workspace` scope; provide `scope`, `name`, `prompt`, and `workspace` for workspace scope. Provider, model, and reasoning are optional agent-level overrides; when omitted, the definition inherits the target project runtime defaults.

Fresh daemon boot registers the operator `$HOME` as the default workspace through the resolver, so `compozy__workspace_list` should return at least that workspace on a clean install.

A successful workspace catalog read reconciles registered roots before returning: entries whose directories no longer exist are durably unregistered, while other filesystem or deletion failures fail the read instead of hiding uncertain state. `compozy__workspace_list`, `compozy workspace list`, and HTTP/UDS `GET /api/workspaces` share this catalog.

Workspace unregister is atomic with credential cleanup: it removes workspace-scoped MCP OAuth rows and their encrypted access/refresh values, preserves global and sibling-workspace credentials, and leaves all state intact when cleanup fails.

Provider model tools: `compozy__provider_models_list`, `compozy__provider_models_curate`, `compozy__provider_models_refresh`, `compozy__provider_models_status`.

`compozy__provider_models_list` accepts `view=curated|all` and defaults to curated; the CLI equivalents are `compozy provider models list` and `compozy provider models list --all`. `compozy__provider_models_curate` is mutating, requires `providers.models.write`, and accepts required `provider_id`/`model_id` plus optional `hidden`, `featured`, `deprecated`, and `default_effort`. Its CLI fallback is `compozy provider models set`. Treat `model_not_found` and `reasoning_effort_unsupported` as terminal input diagnostics; when the descriptor reports the settings backend unavailable, do not retry blindly.
For providers with an explicit curated set, the default view contains visible explicit or featured rows; live-only rows appear there only through the no-explicit-set fallback.
Model-list and curation results may include a `cost` object with independent `input_per_million`, `output_per_million`, `cache_read_per_million`, `cache_write_per_million`, and `reasoning_per_million` fields. A missing field means that bucket is unpriced; never infer it from another field.

## Workspace Boundary

A call that names a workspace other than the bound session's is a cross-workspace request. The session's effective permission mode decides it, and there is no separate toggle, grant, or config key:

- `approve-all`: allowed at every seam.
- `deny-all`: denied at every seam, including native tools. Compozy never prompts for crossing under this mode.
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
- Layouts: `compozy__layout_get`, `compozy__layout_preview`, `compozy__layout_arrange`,
  `compozy__layout_resize`, `compozy__layout_balance`, `compozy__layout_undo`, `compozy__layout_redo`,
  `compozy__layout_export`, `compozy__layout_validate`, `compozy__layout_apply`.

`desktop_switch`, `window_focus`, and `window_zoom` require a connected `client_id`.
`window_navigate` changes presentation only when given one. Preview/validate never write;
desktop delete, window close, and layout apply carry destructive risk. Discover `window_layout`
resources with `compozy__resources_list`; `resource_id` is exclusive with inline arrange fields. CLI
fallbacks are `compozy desktop|window|layout`; read `window-management.md` for multi-step changes.

## Skills And Memory Tools

Skill tools: `compozy__skill_list`, `compozy__skill_search`, `compozy__skill_view`.

Resolve canonical `compozy__skill_view`, then use its returned tool reference with a file/resource argument when reading `skills/compozy/references/*.md` from inside Compozy.

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

Task tools: `compozy__task_list`, `compozy__task_read`, `compozy__task_create`, `compozy__task_child_create`, `compozy__task_update`, `compozy__task_cancel`, `compozy__task_promote_from_thread`, `compozy__task_fanout_runs`, `compozy__task_run_list`, `compozy__task_run_review_request`, `compozy__task_run_review_list`, `compozy__task_run_review_show`, `compozy__task_execution_profile_get`, `compozy__task_execution_profile_set`, `compozy__task_execution_profile_delete`, `compozy__task_notification_subscribe`, `compozy__task_notification_list`, `compozy__task_notification_show`, `compozy__task_notification_delete`.

Session-bound autonomy tools: `compozy__task_run_claim_next`, `compozy__task_run_heartbeat`, `compozy__task_run_complete`, `compozy__task_run_fail`, `compozy__task_run_release`, `compozy__task_run_review_submit`.

Autonomy tools are bound to the caller session. Do not substitute general task mutation tools for session-bound lease operations. Read references/tasks-and-orchestration.md before claiming, heartbeating, completing, failing, releasing, or submitting review verdicts.

## Loop Tools

Toolset `compozy__loops` (16 tools):
`compozy__loop_list/_inspect/_validate/_status/_runs/_create/_run/_configure/_pause/_resume/_approve/_stop/_delete`,
`compozy__goal_get`, `compozy__goal_report`, and `compozy__loop_turns`.

No `compozy__loop_edit`. See references/loops.md for publishing, approval/self-approval, and Goal report
binding semantics.

## Config, Hooks, Automation, Marketplace, Extensions, Bundles, Resources, And MCP Tools

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

`compozy__marketplace_search` returns MCP, extension, skill, and bundle rows. Single-kind cursors bind the
query, scope, workspace, and source projection; grouped searches omit them. Paging and installed
identity rules live in `references/tools-and-skills.md`. Installed state is scoped to the caller's
exact workspace; never reuse it across workspace scopes.
Extension tools are `compozy__extensions_{init,build,validate,dev,reload,logs,list,info,install,update,remove,enable,disable}`;
there is no extension-specific native search tool. `validate` and `logs` are read-only; `init`, `build`,
`dev`, and `reload` are mutating, with `build`, `dev`, and `reload` requiring interaction. Resolve live
schemas first; authoring, workspace binding, generation handles, update/remove commit boundaries, and
cleanup warnings live in `references/capabilities-and-bundles.md`.

The `compozy__automation_jobs_create` and `compozy__automation_jobs_update` descriptors expose the complete recurring schedule shape, including `catch_up_policy` and `misfire_grace_seconds`. Resolve the live descriptor instead of guessing the enum or sending catch-up fields to a one-time `at` schedule.

`compozy__automation_jobs_create` and `compozy__automation_jobs_update` reject Agent prompts and Task descriptions containing command-shaped Compozy daemon restart, stop, or kill instructions before persistence, including resource-applied definitions. The tool error names `compozy_daemon`, `process_signal`, or `service_manager`; remove the lifecycle command before retrying. There is no bypass.

Bundle tools live under `compozy__bundles_*` for list/info/activate/deactivate/status. Resource tools live under `compozy__resources_*` for list/info/snapshot of desired-state resources.

MCP diagnostics are `compozy__mcp_status` and `compozy__mcp_auth_status`. A
workspace-scoped server with five consecutive confirmed permanent failures reports `state: "dead"`
from `compozy__mcp_status`; its nested runtime reason is `backend_dead`. During the same daemon lifetime,
resolve its last-known tools through `compozy__tool_info`, which retains their unavailable descriptors
and diagnostic instead of hiding them. Do not retry a dead tool blindly or invent a revive call: Compozy
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

## Descriptor And Skill Co-Ship

Changing native tools is a public agent contract change. When a Compozy change adds, removes, renames, or changes a `compozy__*` ID, toolset, descriptor, schema digest, risk flag, availability diagnostic, capability gate, or CLI/API fallback, update `skills/compozy/` in the same change or record explicit no-impact evidence.
