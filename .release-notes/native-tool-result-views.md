---
title: Native tools return compact records by default
type: fix
breaking: true
---

99 native tools now return a compact record by default. Pass `result_view: "full"` for
the complete record. The other 189 tools return a single result and reject the selector.
Identity, follow-up handles, failures, stop reasons, pending interactions and pagination
remain in compact. Requested content is never trimmed to meet a size target.

## Migration notes

This is the operator-approved hard cut in #1432. There is no global default override.
HTTP/UDS session responses and `compozy session` output keep their shape.

- Agent prompt: instruct the agent to pass `result_view: "full"` on calls reading moved fields.
- Loop node: add literal `result_view: full` to `params`. Validation reports
  `result_view_field_requires_full` with the source node and path; templates fail with
  `result_view_not_literal`. Review edits cannot change the selected view.
- Hook: when `has_result_views` is true and `pinned_result_view` is absent, patch
  `tool_input` in `tool.pre_call` to include `result_view: "full"`. The agent receives
  full too. Overriding a Loop pin fails with `tool_denied` / `result_view_pinned`.
  `tool.post_call.result_view` reports the returned view.
- Managed-session MCP client: send `"result_view":"full"` in call `arguments`.
  Advertised `outputSchema` accepts compact, full, continuation envelope and typed error.
  `compozy mcp serve` has a separate `compozy_host__*` Host API projection without views.
- CLI script: put `"result_view":"full"` in `compozy tool invoke --input`.

Discover both schemas and digests through `compozy__tool_info` or `compozy tool info`.
`output_schema` remains the compact contract. Invalid or non-string selectors, and a
selector on a single-result native tool, fail with `tool_invalid_input` (or the tool's
existing `invalid_request`) / `schema_invalid`, before execution. Extension and MCP
sidecar tools retain their own input semantics.

### Defaults that changed

The following inventory names every tool with a changed default and the fields moved to full.
The full result preserves the previous complete record.
For open historical full schemas, inspect a real full result to discover undeclared fields.

`compozy__session_archive`, `compozy__session_continue`, `compozy__session_create`,
`compozy__session_describe`, `compozy__session_fork`, `compozy__session_later`,
`compozy__session_list`, `compozy__session_mark_seen`, `compozy__session_move_to_profile`,
`compozy__session_rename`, `compozy__session_reparent`, `compozy__session_resume_later`,
`compozy__session_rewind`, `compozy__session_runtime_clear`, `compozy__session_runtime_set`,
`compozy__session_status`, `compozy__session_unarchive`, `compozy__session_worktree_bind`,
`compozy__session_worktree_unbind`:

Shared record (under `session`, `sessions[]` or `original`): `activity`, `attach_expires_at`,
`attachable`, `attached_to`, `attention_changed_at`, `available_commands`, `busy_input`,
`derivation`, `diagnostics`, `effective_permissions`, `effective_workspace_access`, `escalated`,
`folder_id`, `health`, `instance_id`, `linked_work`, `monitor_summary`, `parent_title`,
`process_summary`, `profile_archived`, `profile_color`, `profile_emoji`, `profile_icon`,
`profile_name`, `pull_request_lookup`, `reply_watches`, `returned_from_later_at`, `scope`,
`supervision`, `verified`, `working_dir`, `workspace_path`, `worktree_binding`, `runtime.acp_caps`,
`runtime.acp_session_id`, `runtime.last_known`, `runtime.recovery`, `runtime.requested`,
`runtime.selected`, `runtime.transition`, `lineage.auto_stop_on_parent`, `lineage.notify_creator`,
`lineage.origin_agent_name`, `lineage.origin_message_id`, `lineage.permission_policy`,
`lineage.spawn_depth`, `lineage.spawn_role`. Optional fields move only when the full record carries
them. `session_list` retains `sessions[].folder_id` and `sessions[].health`.

`compozy__agent_create`:

`agent.acp_options`, `agent.appearance`, `agent.auto_compact_window`, `agent.command`,
`agent.deny_tools`, `agent.display_name`, `agent.effective_runtime.acp_options`,
`agent.effective_runtime.auto_compact_window`, `agent.effective_runtime.lean_environment`,
`agent.effective_runtime.policy_excluded_acp_options`, `agent.effective_runtime.session_env`,
`agent.effective_runtime.session_env_status`, `agent.effective_runtime.sources`,
`agent.fallback_chain`, `agent.lean_environment`, `agent.mcp_servers`, `agent.model`,
`agent.permission_rules`, `agent.permissions`, `agent.prompt`, `agent.provider`,
`agent.reasoning_effort`, `agent.resolved_appearance`, `agent.session_approval`,
`agent.session_env`, `agent.shadows`, `agent.skills`, `agent.speed`, `agent.tools`,
`agent.toolsets`, `agent.workspace_access`

`compozy__agent_heartbeat_sources`:

`kinds[].params_schema`, `scripts[].approved_sha256`, `scripts[].last_modified`, `scripts[].line_count`, `scripts[].path`, `scripts[].sha256`

`compozy__agent_heartbeat_status`:

`heartbeat.config_digest`, `heartbeat.digest`, `heartbeat.preferences`, `heartbeat.source_path`

`compozy__agent_instance_archive`, `compozy__agent_instance_checks`,
`compozy__agent_instance_create`, `compozy__agent_instance_ensure_main`,
`compozy__agent_instance_get`, `compozy__agent_instance_pause`, `compozy__agent_instance_resume`:

`instance.created_at`, `instance.last_wake_at`, `instance.next_wake_at`, `instance.now`,
`instance.provider`, `instance.quiet_streak`, `instance.resolved_appearance`,
`instance.sources[].last_item_count`, `instance.sources[].last_polled_at`,
`instance.sources[].last_success_at`, `instance.sources[].script_path`,
`instance.sources[].script_sha256`, `instance.updated_at`

`compozy__agent_instance_list`:

`instances[].created_at`, `instances[].last_wake_at`, `instances[].next_wake_at`, `instances[].now`,
`instances[].provider`, `instances[].quiet_streak`, `instances[].resolved_appearance`,
`instances[].sources[].last_item_count`, `instances[].sources[].last_polled_at`,
`instances[].sources[].last_success_at`, `instances[].sources[].script_path`,
`instances[].sources[].script_sha256`, `instances[].updated_at`

`compozy__agent_list`:

`agents[].acp_options`, `agents[].appearance`, `agents[].auto_compact_window`, `agents[].command`,
`agents[].deny_tools`, `agents[].display_name`, `agents[].effective_runtime.acp_options`,
`agents[].effective_runtime.auto_compact_window`, `agents[].effective_runtime.lean_environment`,
`agents[].effective_runtime.policy_excluded_acp_options`, `agents[].effective_runtime.session_env`,
`agents[].effective_runtime.session_env_status`, `agents[].effective_runtime.sources`,
`agents[].fallback_chain`, `agents[].lean_environment`, `agents[].mcp_servers`, `agents[].model`,
`agents[].permission_rules`, `agents[].permissions`, `agents[].prompt`, `agents[].provider`,
`agents[].reasoning_effort`, `agents[].resolved_appearance`, `agents[].session_approval`,
`agents[].session_env`, `agents[].shadows`, `agents[].skills`, `agents[].speed`, `agents[].tools`,
`agents[].toolsets`, `agents[].workspace_access`

`compozy__automation_jobs_create`, `compozy__automation_jobs_disable`,
`compozy__automation_jobs_enable`, `compozy__automation_jobs_get`,
`compozy__automation_jobs_update`:

`job.created_at`, `job.fire_limit`, `job.loop_target.input_mapping`, `job.loop_target.inputs`,
`job.profile_color`, `job.profile_icon`, `job.prompt`, `job.retry`, `job.scheduler.catch_up_policy`,
`job.scheduler.job_id`, `job.scheduler.last_fire_id`, `job.scheduler.last_run_at`,
`job.scheduler.last_scheduled_at`, `job.scheduler.misfire_grace_seconds`,
`job.scheduler.next_run_at`, `job.scheduler.updated_at`, `job.task.description`, `job.updated_at`,
`job.task.owner`

`compozy__automation_jobs_list`:

`jobs[].created_at`, `jobs[].fire_limit`, `jobs[].loop_target.input_mapping`,
`jobs[].loop_target.inputs`, `jobs[].profile_color`, `jobs[].profile_icon`, `jobs[].prompt`,
`jobs[].retry`, `jobs[].scheduler.catch_up_policy`, `jobs[].scheduler.job_id`,
`jobs[].scheduler.last_fire_id`, `jobs[].scheduler.last_run_at`,
`jobs[].scheduler.last_scheduled_at`, `jobs[].scheduler.misfire_grace_seconds`,
`jobs[].scheduler.next_run_at`, `jobs[].scheduler.updated_at`, `jobs[].task.description`,
`jobs[].updated_at`, `jobs[].task.owner`

`compozy__automation_suggestions_accept`:

`job.created_at`, `job.fire_limit`, `job.loop_target.input_mapping`, `job.loop_target.inputs`,
`job.profile_color`, `job.profile_icon`, `job.prompt`, `job.retry`, `job.scheduler.catch_up_policy`,
`job.scheduler.job_id`, `job.scheduler.last_fire_id`, `job.scheduler.last_run_at`,
`job.scheduler.last_scheduled_at`, `job.scheduler.misfire_grace_seconds`,
`job.scheduler.next_run_at`, `job.scheduler.updated_at`, `job.task.description`, `job.updated_at`,
`suggestion.payload.created_at`, `suggestion.payload.fire_limit`,
`suggestion.payload.loop_target.input_mapping`, `suggestion.payload.loop_target.inputs`,
`suggestion.payload.profile_color`, `suggestion.payload.profile_icon`, `suggestion.payload.prompt`,
`suggestion.payload.retry`, `suggestion.payload.scheduler.catch_up_policy`,
`suggestion.payload.scheduler.job_id`, `suggestion.payload.scheduler.last_fire_id`,
`suggestion.payload.scheduler.last_run_at`, `suggestion.payload.scheduler.last_scheduled_at`,
`suggestion.payload.scheduler.misfire_grace_seconds`, `suggestion.payload.scheduler.next_run_at`,
`suggestion.payload.scheduler.updated_at`, `suggestion.payload.task.description`,
`suggestion.payload.updated_at`, `suggestion.payload.task.owner`, `job.task.owner`

`compozy__automation_suggestions_dismiss`:

`suggestion.payload.created_at`, `suggestion.payload.fire_limit`,
`suggestion.payload.loop_target.input_mapping`, `suggestion.payload.loop_target.inputs`,
`suggestion.payload.profile_color`, `suggestion.payload.profile_icon`, `suggestion.payload.prompt`,
`suggestion.payload.retry`, `suggestion.payload.scheduler.catch_up_policy`,
`suggestion.payload.scheduler.job_id`, `suggestion.payload.scheduler.last_fire_id`,
`suggestion.payload.scheduler.last_run_at`, `suggestion.payload.scheduler.last_scheduled_at`,
`suggestion.payload.scheduler.misfire_grace_seconds`, `suggestion.payload.scheduler.next_run_at`,
`suggestion.payload.scheduler.updated_at`, `suggestion.payload.task.description`,
`suggestion.payload.updated_at`, `suggestion.payload.task.owner`

`compozy__automation_suggestions_list`:

`suggestions[].payload.created_at`, `suggestions[].payload.fire_limit`,
`suggestions[].payload.loop_target.input_mapping`, `suggestions[].payload.loop_target.inputs`,
`suggestions[].payload.profile_color`, `suggestions[].payload.profile_icon`,
`suggestions[].payload.prompt`, `suggestions[].payload.retry`,
`suggestions[].payload.scheduler.catch_up_policy`, `suggestions[].payload.scheduler.job_id`,
`suggestions[].payload.scheduler.last_fire_id`, `suggestions[].payload.scheduler.last_run_at`,
`suggestions[].payload.scheduler.last_scheduled_at`,
`suggestions[].payload.scheduler.misfire_grace_seconds`,
`suggestions[].payload.scheduler.next_run_at`, `suggestions[].payload.scheduler.updated_at`,
`suggestions[].payload.task.description`, `suggestions[].payload.updated_at`,
`suggestions[].payload.task.owner`

`compozy__automation_triggers_create`, `compozy__automation_triggers_disable`,
`compozy__automation_triggers_enable`, `compozy__automation_triggers_get`,
`compozy__automation_triggers_update`:

`trigger.created_at`, `trigger.filter`, `trigger.fire_limit`, `trigger.loop_target.input_mapping`,
`trigger.loop_target.inputs`, `trigger.profile_color`, `trigger.profile_icon`, `trigger.prompt`,
`trigger.retry`, `trigger.updated_at`, `trigger.webhook_secret_hash`

`compozy__automation_triggers_list`:

`triggers[].created_at`, `triggers[].filter`, `triggers[].fire_limit`,
`triggers[].loop_target.input_mapping`, `triggers[].loop_target.inputs`, `triggers[].profile_color`,
`triggers[].profile_icon`, `triggers[].prompt`, `triggers[].retry`, `triggers[].updated_at`,
`triggers[].webhook_secret_hash`

`compozy__cmd_palette_list`:

`commands[].action`, `commands[].alias`, `commands[].availability_exempt`, `commands[].bindings`,
`commands[].execution`, `commands[].global_shortcut`, `commands[].icon`, `commands[].keywords`,
`commands[].section`, `commands[].when`

`compozy__command_list`:

`commands[].description`, `commands[].placements`

`compozy__desktop_clients`:

`clients[].connected_at`, `clients[].focus_order`, `clients[].global_shortcuts`, `clients[].palette_context`, `clients[].stack_active`

`compozy__extensions_build`:

`build.manifest.capabilities`, `build.manifest.description`, `build.manifest.gateway`,
`build.manifest.inputs`, `build.manifest.min_compozy_version`, `build.manifest.permissions`,
`build.manifest.profiles`, `build.manifest.requires_env`, `build.manifest.resources`,
`build.manifest.subprocess`

`compozy__extensions_dev`, `compozy__extensions_info`, `compozy__extensions_install`, `compozy__extensions_reload`:

`extension.bound_env_keys`, `extension.capabilities`, `extension.contents`, `extension.description`,
`extension.diagnostics[].data_freshness`, `extension.diagnostics[].doc_url`,
`extension.diagnostics[].evidence`, `extension.layout`, `extension.marketplace`,
`extension.permissions`, `extension.pid`, `extension.provenance`, `extension.requires_env`,
`extension.trust.warnings[].data_freshness`, `extension.trust.warnings[].doc_url`,
`extension.trust.warnings[].evidence`, `extension.uptime_seconds`

`compozy__extensions_list`:

`extensions[].bound_env_keys`, `extensions[].capabilities`, `extensions[].contents`,
`extensions[].description`, `extensions[].diagnostics[].data_freshness`,
`extensions[].diagnostics[].doc_url`, `extensions[].diagnostics[].evidence`, `extensions[].layout`,
`extensions[].marketplace`, `extensions[].permissions`, `extensions[].pid`,
`extensions[].provenance`, `extensions[].requires_env`,
`extensions[].trust.warnings[].data_freshness`, `extensions[].trust.warnings[].doc_url`,
`extensions[].trust.warnings[].evidence`, `extensions[].uptime_seconds`

`compozy__goal_control`:

`snapshot.context.nudge_ratio`, `snapshot.context.ratio`, `snapshot.context.reported_at`, `snapshot.context.size`, `snapshot.context.used`, `snapshot.contract_summary`, `snapshot.objective`

`compozy__goal_get`:

`goal.context.nudge_ratio`, `goal.context.ratio`, `goal.context.reported_at`, `goal.context.size`, `goal.context.used`, `goal.contract_summary`, `goal.objective`

`compozy__layout_get`, `compozy__layout_preview`:

`snapshot.closed_entries`, `snapshot.history`

`compozy__loop_configure`, `compozy__loop_inspect`:

`effective_config`

`compozy__loop_create`:

`loop.catalog`, `loop.definition`, `loop.effective_config`, `loop.effective_lifecycle`

`compozy__loop_fork`, `compozy__loop_run`:

`run.inputs`, `run.profile_color`, `run.profile_icon`, `run.profile_name`, `run.reattempt_strategy`,
`run.start_metadata`, `run.started_by_kind`, `run.started_by_ref`, `run.started_origin_kind`,
`run.started_origin_ref`

`compozy__loop_list`:

`loops[].catalog`, `loops[].contract`, `loops[].inputs`, `loops[].start`

`compozy__loop_node_amend`:

`amendment.amended`, `amendment.original`

`compozy__loop_nodes`:

`items[].output.resolved_runtime`

`compozy__loop_requests`:

`items[].context`, `items[].edit_schema`, `items[].expect`, `items[].proposed_preview`, `items[].respond_schema`

`compozy__loop_runs`:

`runs[].inputs`, `runs[].profile_color`, `runs[].profile_icon`, `runs[].profile_name`,
`runs[].reattempt_strategy`, `runs[].start_metadata`, `runs[].started_by_kind`,
`runs[].started_by_ref`, `runs[].started_origin_kind`, `runs[].started_origin_ref`

`compozy__loop_status`:

`amendments[].amended`, `amendments[].original`, `effective_config`, `executed_definition`, `generations[].outputs[].resolved_runtime`, `generations[].verdicts[].criteria[].evidence`, `generations[].verdicts[].criteria[].payload`, `materialized_contract`, `requests[].context`, `requests[].edit_schema`, `requests[].expect`, `requests[].proposed_preview`, `requests[].respond_schema`, `run.inputs`, `run.profile_color`, `run.profile_icon`, `run.profile_name`, `run.reattempt_strategy`, `run.start_metadata`, `run.started_by_kind`, `run.started_by_ref`, `run.started_origin_kind`, `run.started_origin_ref`

`compozy__marketplace_search`:

`items[].category_label`, `items[].icon`, `items[].manage_path`, `items[].published_at`,
`items[].trust.warnings[].data_freshness`, `items[].trust.warnings[].doc_url`,
`items[].trust.warnings[].evidence`, `items[].updated_at`

`compozy__marketplace_sources`:

`sources[].diagnostics[].data_freshness`, `sources[].diagnostics[].doc_url`, `sources[].diagnostics[].evidence`

`compozy__observe_metrics`:

`health.activities[].elapsed_ms`, `health.activities[].elapsed_seconds`,
`health.activities[].idle_seconds`, `health.activities[].last_activity_at`,
`health.activities[].last_activity_detail`, `health.activities[].last_activity_kind`,
`health.activities[].last_progress_at`, `health.activities[].turn_source`,
`health.activities[].turn_started_at`, `health.agent_probes[].checked_at`,
`health.agent_probes[].command`, `health.agent_probes[].duration_ms`,
`health.agent_probes[].executable`, `health.global_db_size_bytes`,
`health.persistence.global_db_size_bytes`, `health.persistence.session_db_size_bytes`,
`health.retention.deleted_event_summaries`, `health.retention.deleted_permission_log_rows`,
`health.retention.deleted_token_stats`, `health.retention.deleted_token_usage_daily`,
`health.retention.last_cutoff_at`, `health.retention.last_sweep_at`,
`health.retention.retention_days`, `health.retention.sweep_interval_seconds`,
`health.session_db_size_bytes`

`compozy__provider_models_curate`:

`apply.agent_name`, `apply.profile`, `apply.scope`, `apply.section`, `apply.workspace_id`, `apply.write_target`, `model.config_options`, `model.context_window`, `model.cost`, `model.max_input_tokens`, `model.max_output_tokens`, `model.reasoning_apply`, `model.reasoning_known`, `model.reasoning_source`, `model.refreshed_at`, `model.release_date`, `model.sources[].priority`, `model.sources[].refreshed_at`

`compozy__provider_models_list`:

`models[].config_options`, `models[].context_window`, `models[].cost`, `models[].max_input_tokens`,
`models[].max_output_tokens`, `models[].reasoning_apply`, `models[].reasoning_known`,
`models[].reasoning_source`, `models[].refreshed_at`, `models[].release_date`,
`models[].sources[].priority`, `models[].sources[].refreshed_at`

`compozy__resources_list`, `compozy__resources_snapshot`:

`records[].spec`

`compozy__session_spawn`:

`permissions`

`compozy__skill_list`, `compozy__skill_search`:

`skills[].description`, `skills[].dir`, `skills[].metadata`, `skills[].provenance`;
`skills[].diagnostics` when all entries are healthy (see below)

`compozy__subagent_capabilities`:

`providers[].models`

`compozy__task_cancel`, `compozy__task_child_create`, `compozy__task_create`, `compozy__task_update`:

`task.auto_enqueue_on_ready`, `task.created_by`, `task.description`, `task.grant_ceiling`,
`task.metadata`, `task.origin`, `task.wake_creator`

`compozy__task_fanout_runs`, `compozy__task_run_list`:

`runs[].claim_token_hash`, `runs[].idempotency_key`, `runs[].metadata`, `runs[].origin`,
`runs[].preferred_capabilities`, `runs[].required_capabilities`, `runs[].result_bytes`,
`runs[].result_ref`

`compozy__task_list`:

`tasks[].auto_enqueue_on_ready`, `tasks[].created_by`, `tasks[].origin`, `tasks[].profile_color`,
`tasks[].profile_icon`, `tasks[].profile_name`, `tasks[].wake_creator`

`compozy__task_read`:

`task.children[].auto_enqueue_on_ready`, `task.children[].created_by`, `task.children[].origin`,
`task.children[].profile_color`, `task.children[].profile_icon`, `task.children[].profile_name`,
`task.children[].wake_creator`, `task.designation_rollups`, `task.events`,
`task.runs[].claim_token_hash`, `task.runs[].designation`, `task.runs[].idempotency_key`,
`task.runs[].metadata`, `task.runs[].origin`, `task.runs[].profile_color`,
`task.runs[].profile_icon`, `task.runs[].profile_name`, `task.summary.auto_enqueue_on_ready`,
`task.summary.created_by`, `task.summary.origin`, `task.summary.profile_color`,
`task.summary.profile_icon`, `task.summary.profile_name`, `task.summary.wake_creator`,
`task.task.auto_enqueue_on_ready`, `task.task.created_by`, `task.task.origin`,
`task.task.profile_color`, `task.task.profile_icon`, `task.task.profile_name`,
`task.task.wake_creator`

`compozy__task_recover`:

`task.auto_enqueue_on_ready`, `task.created_by`, `task.description`, `task.metadata`, `task.origin`,
`task.profile_color`, `task.profile_icon`, `task.profile_name`, `task.wake_creator`

`compozy__task_run_review_list`:

`reviews[].created_at`, `reviews[].review_text`, `reviews[].reviewed_by`, `reviews[].routed_at`,
`reviews[].started_at`, `reviews[].updated_at`

`compozy__task_run_review_request`:

`review.created_at`, `review.review_text`, `review.reviewed_by`, `review.routed_at`, `review.started_at`, `review.updated_at`

`compozy__task_run_review_submit`:

`continuation_run.claim_token_hash`, `continuation_run.idempotency_key`, `continuation_run.metadata`, `continuation_run.origin`, `continuation_run.preferred_capabilities`, `continuation_run.required_capabilities`, `continuation_run.result_bytes`, `continuation_run.result_ref`, `review.created_at`, `review.review_text`, `review.reviewed_by`, `review.routed_at`, `review.started_at`, `review.updated_at`

`compozy__tool_approvals_list`:

`grants[].profile_color`, `grants[].profile_emoji`, `grants[].profile_icon`, `grants[].profile_name`

`compozy__tool_approvals_set`:

`grant.profile_color`, `grant.profile_emoji`, `grant.profile_icon`, `grant.profile_name`

`compozy__tool_list`:

`tools[].decision.agent_policy_result`, `tools[].decision.availability_result`,
`tools[].decision.hook_result`, `tools[].decision.registry_policy_result`,
`tools[].decision.session_policy_result`, `tools[].decision.source_policy_result`,
`tools[].decision.system_permission_mode`, `tools[].decision.visible_to_operator`,
`tools[].decision.visible_to_session`, `tools[].descriptor.concurrency_safe`,
`tools[].descriptor.destructive`, `tools[].descriptor.input_schema_digest`,
`tools[].descriptor.max_result_bytes`, `tools[].descriptor.open_world`,
`tools[].descriptor.output_schema_digest`, `tools[].descriptor.read_only`,
`tools[].descriptor.requires_interaction`, `tools[].descriptor.toolsets`

`compozy__tool_search`:

`tools[].decision.agent_policy_result`, `tools[].decision.availability_result`,
`tools[].decision.hook_result`, `tools[].decision.registry_policy_result`,
`tools[].decision.session_policy_result`, `tools[].decision.source_policy_result`,
`tools[].decision.system_permission_mode`, `tools[].decision.visible_to_operator`,
`tools[].decision.visible_to_session`, `tools[].descriptor.access`, `tools[].descriptor.backend`,
`tools[].descriptor.check_in_writable`, `tools[].descriptor.concurrency_safe`,
`tools[].descriptor.destructive`, `tools[].descriptor.display_title`,
`tools[].descriptor.friendly_verb`, `tools[].descriptor.idempotent`,
`tools[].descriptor.input_error_code`, `tools[].descriptor.input_schema`,
`tools[].descriptor.input_schema_digest`, `tools[].descriptor.max_result_bytes`,
`tools[].descriptor.open_world`, `tools[].descriptor.output_schema`,
`tools[].descriptor.output_schema_digest`, `tools[].descriptor.output_views.compact.schema`,
`tools[].descriptor.output_views.full.schema`, `tools[].descriptor.preview`,
`tools[].descriptor.read_only`, `tools[].descriptor.requires_interaction`,
`tools[].descriptor.reserved_workspace_partition`, `tools[].descriptor.search_hints`,
`tools[].descriptor.source`, `tools[].descriptor.tags`, `tools[].descriptor.toolsets`,
`tools[].descriptor.visibility`

`compozy__workspace_describe`:

`agents[].acp_options`, `agents[].appearance`, `agents[].auto_compact_window`,
`agents[].category_path`, `agents[].command`, `agents[].definition_digest`, `agents[].deny_tools`,
`agents[].effective_runtime`, `agents[].fallback_chain`, `agents[].label`,
`agents[].lean_environment`, `agents[].mcp_servers`, `agents[].permission_rules`,
`agents[].permissions`, `agents[].prompt`, `agents[].resolved_appearance`,
`agents[].session_approval`, `agents[].session_env`, `agents[].shadows`, `agents[].skills`,
`agents[].tools`, `agents[].toolsets`, `agents[].workspace_access`, `providers[].auth_mode`,
`providers[].env_policy`, `providers[].harness`, `providers[].home_policy`,
`providers[].runtime_live_apply`, `providers[].runtime_provider`, `providers[].runtime_strategy`,
`sessions[].activity`, `sessions[].attach_expires_at`, `sessions[].attachable`,
`sessions[].attached_to`, `sessions[].attention_changed_at`, `sessions[].available_commands`,
`sessions[].busy_input`, `sessions[].derivation`, `sessions[].diagnostics`,
`sessions[].effective_permissions`, `sessions[].effective_workspace_access`, `sessions[].escalated`,
`sessions[].folder_id`, `sessions[].health`, `sessions[].instance_id`,
`sessions[].lineage.auto_stop_on_parent`, `sessions[].lineage.notify_creator`,
`sessions[].lineage.origin_agent_name`, `sessions[].lineage.origin_message_id`,
`sessions[].lineage.permission_policy`, `sessions[].lineage.spawn_depth`,
`sessions[].lineage.spawn_role`, `sessions[].linked_work`, `sessions[].monitor_summary`,
`sessions[].parent_title`, `sessions[].process_summary`, `sessions[].profile_archived`,
`sessions[].profile_color`, `sessions[].profile_emoji`, `sessions[].profile_icon`,
`sessions[].profile_name`, `sessions[].pull_request_lookup`, `sessions[].reply_watches`,
`sessions[].returned_from_later_at`, `sessions[].runtime.acp_caps`,
`sessions[].runtime.acp_session_id`, `sessions[].runtime.last_known`, `sessions[].runtime.recovery`,
`sessions[].runtime.requested`, `sessions[].runtime.selected`, `sessions[].runtime.transition`,
`sessions[].scope`, `sessions[].supervision`, `sessions[].verified`, `sessions[].working_dir`,
`sessions[].workspace_path`, `sessions[].worktree_binding`, `skills[].dir`

`compozy__worktree_list`:

`worktrees[].base_ref`, `worktrees[].created_at`, `worktrees[].created_head`,
`worktrees[].profile_color`, `worktrees[].profile_emoji`, `worktrees[].profile_icon`,
`worktrees[].profile_name`, `worktrees[].run_namespace`, `worktrees[].updated_at`

Compact session records retain `stop_cause`, `permission_mode_degraded`,
`attention_dismissible`, `lineage.ttl_expires_at` and `lineage.spawn_budget`.
Compact heartbeat status retains policy `snapshot_id`. Approval grants retain `profile_archived`;
task review records retain `delivery_id`; Loop status retains gate criterion prompts.
Skill list/search omit diagnostics only when every entry is valid and verification passed,
with no warnings, failure or activation reasons; otherwise the entire diagnostic list stays.

### Schema corrections and existing runs

Eight declared schemas now accept fields already returned: `loop_runs`
(`aggregates.historical`); `loop_status` (`generations[].outputs[].session_id`,
`generations[].verdicts[].item_index`,
`requests[].profile_id`, `requests[].workspace_id`); `loop_request` and `loop_requests`
(`profile_id`, `workspace_id`); `loop_nodes` (`items[].workspace_id`,
`items[].output.session_id`); `cmd_palette_invoke` (`profile_lens`);
`tool_approvals_set` and `tool_approvals_list` (`access_scope`). Only `loop_request`
and `cmd_palette_invoke` change single-result output digests; their input digests stay.

Pre-view Loop snapshots translate to the full contract for this release, including the
superseded digests of those eight tools. The translation and superseded-digest table
are removed in the next release after this one (#1462). Existing YAML is not rewritten.
Automation job/trigger schemas do not reference native tool result fields.

The PR inventory records every tool, classification reason and fixture sizes. Those
examples are not production averages. Hosted tools/list with 99 tools with views is
1,087,936 bytes (the earlier four-view measurement was 755,769 bytes; pre-adoption was
744,687 bytes): 343,249 additional bytes, or 46.1%. The hosted proxy sends both output
schemas in `tools/list` to the managed provider’s MCP client. This is discovery traffic,
not a measured model-token count; model exposure depends on the provider adapter.
The separate Host API projection is unchanged.
