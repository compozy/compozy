# SQLite schema and query coverage audit

Baseline: `84549c6e1fbd691611518df036d3b6165e6ce37c`. Read-only source audit; experiments use isolated in-memory SQLite 3.53.4, never operator databases. All current declarative tables, indexes, foreign keys, and static sqlc queries were enumerated. Timing evidence below is synthetic warm-cache evidence, not a production latency claim. Parent DELETE query plans include foreign-key work; INSERT/UPSERT plans may include unreachable or equality-skipped checks and are not alone evidence of a paid cost.

## Coverage summary

| Stream | Tables | Explicit indexes | Static queries explained | Explanation errors |
| --- | ---: | ---: | ---: | ---: |
| global | 132 | 227 | 692 | 0 |
| session | 10 | 13 | 46 | 0 |
| workspace | 3 | 12 | 16 | 0 |

Resource kernel duplicates two global declarative tables in `internal/resources/schema.go` for standalone raw-resource users: `resource_records` and `resource_source_state`. Their `(kind,id)` and `(source_kind,source_id)` primary keys and scope/owner/source indexes cover current equality boundaries. `idx_resource_kind` is a strict prefix duplicate of the `(kind,id)` primary key, but no removal is proposed without measuring standalone consumers and generator parity. No additional SQLite schema-owning production package was found outside `internal/store` and `internal/resources`.

## Global table inventory

Each table lists every explicit and automatic index, static query reference count (dynamic repositories are reviewed separately), and foreign-key probes that still scan. An index on either member of a composite child key can bound the probe and therefore is not classified as missing merely because the complete key is absent. Partial indexes qualify only when the FK equality implies their predicate.

| Table | Indexes (key columns; partial predicates retained) | Static query references | FK probes without index search |
| --- | --- | ---: | --- |
| `agent_heartbeat_revisions` | `CREATE INDEX idx_agent_heartbeat_revisions_agent_created ON agent_heartbeat_revisions(workspace_id, agent_name, created_at DESC, id DESC)`<br>`sqlite_autoindex_agent_heartbeat_revisions_1(id)` | 4 | `new_snapshot_id → agent_heartbeat_snapshots` |
| `agent_heartbeat_snapshots` | `CREATE INDEX idx_agent_heartbeat_snapshots_agent_created ON agent_heartbeat_snapshots(workspace_id, agent_name, created_at DESC, id DESC)`<br>`sqlite_autoindex_agent_heartbeat_snapshots_2(workspace_id,agent_name,digest)`<br>`sqlite_autoindex_agent_heartbeat_snapshots_1(id)` | 3 | none |
| `agent_heartbeat_wake_events` | `CREATE INDEX idx_agent_heartbeat_wake_events_expires ON agent_heartbeat_wake_events(expires_at)`<br>`CREATE INDEX idx_agent_heartbeat_wake_events_agent_created ON agent_heartbeat_wake_events(workspace_id, agent_name, created_at DESC, id DESC)`<br>`sqlite_autoindex_agent_heartbeat_wake_events_1(id)` | 4 | `policy_snapshot_id → agent_heartbeat_snapshots`<br>`session_id → sessions` |
| `agent_heartbeat_wake_state` | `CREATE INDEX idx_agent_heartbeat_wake_state_next_allowed ON agent_heartbeat_wake_state(next_allowed_at, updated_at DESC)`<br>`sqlite_autoindex_agent_heartbeat_wake_state_1(workspace_id,agent_name,session_id)` | 2 | `policy_snapshot_id → agent_heartbeat_snapshots`<br>`session_id → sessions` |
| `agent_soul_revisions` | `CREATE INDEX idx_agent_soul_revisions_agent ON agent_soul_revisions(workspace_id, agent_name, created_at DESC)`<br>`sqlite_autoindex_agent_soul_revisions_1(id)` | 4 | none |
| `agent_soul_snapshots` | `CREATE INDEX idx_agent_soul_snapshots_agent ON agent_soul_snapshots(workspace_id, agent_name, created_at DESC)`<br>`sqlite_autoindex_agent_soul_snapshots_2(workspace_id,agent_name,digest)`<br>`sqlite_autoindex_agent_soul_snapshots_1(id)` | 3 | none |
| `app_metadata` | `sqlite_autoindex_app_metadata_1(key)` | 4 | none |
| `attention_acknowledgements` | `sqlite_autoindex_attention_acknowledgements_1(profile_id,actor_kind,actor_id,occurrence_id)` | 2 | none |
| `attention_snapshots` | `CREATE INDEX attention_snapshots_expiry_idx ON attention_snapshots(expires_at)`<br>`sqlite_autoindex_attention_snapshots_1(id)` | 3 | `profile_id → profiles` |
| `attention_workspace_mutes` | `sqlite_autoindex_attention_workspace_mutes_1(profile_id,workspace_id)` | 4 | `workspace_id → workspaces` |
| `automation_job_catalog_entries` | `CREATE INDEX idx_automation_job_catalog_workspace_order ON automation_job_catalog_entries(workspace_id, source_rank, name, job_id)`<br>`CREATE INDEX idx_automation_job_catalog_order ON automation_job_catalog_entries(source_rank, name, job_id)`<br>`sqlite_autoindex_automation_job_catalog_entries_1(job_id)` | 1 | none |
| `automation_job_overlays` | `sqlite_autoindex_automation_job_overlays_1(job_id)` | 5 | none |
| `automation_jobs` | `CREATE UNIQUE INDEX uq_automation_jobs_workspace_name ON automation_jobs(workspace_id, name) WHERE scope = 'workspace'`<br>`CREATE UNIQUE INDEX uq_automation_jobs_global_name ON automation_jobs(name) WHERE scope = 'global'`<br>`CREATE INDEX idx_automation_jobs_loop_target ON automation_jobs(loop_name, loop_workspace_id) WHERE target_kind = 'loop'`<br>`CREATE INDEX idx_automation_jobs_profile ON automation_jobs(profile_id, id)`<br>`CREATE INDEX idx_automation_jobs_enabled ON automation_jobs(enabled)`<br>`sqlite_autoindex_automation_jobs_1(id)` | 5 | `loop_workspace_id → workspaces`<br>`workspace_id → workspaces` |
| `automation_runs` | `CREATE INDEX idx_automation_runs_trigger_latest ON automation_runs(trigger_id, started_at DESC, id DESC) WHERE trigger_id IS NOT NULL`<br>`CREATE INDEX idx_automation_runs_job_latest ON automation_runs(job_id, started_at DESC, id DESC) WHERE job_id IS NOT NULL`<br>`CREATE UNIQUE INDEX uq_automation_runs_fire_id ON automation_runs(fire_id) WHERE fire_id IS NOT NULL`<br>`CREATE INDEX idx_automation_runs_trigger ON automation_runs(trigger_id)`<br>`CREATE INDEX idx_automation_runs_status ON automation_runs(status)`<br>`CREATE INDEX idx_automation_runs_started ON automation_runs(started_at)`<br>`CREATE INDEX idx_automation_runs_loop_run ON automation_runs(loop_run_id)`<br>`CREATE INDEX idx_automation_runs_job ON automation_runs(job_id)`<br>`sqlite_autoindex_automation_runs_1(id)` | 7 | `profile_id → profiles` |
| `automation_scheduler_state` | `CREATE INDEX idx_automation_scheduler_next_run ON automation_scheduler_state(next_run_at)`<br>`CREATE INDEX idx_automation_scheduler_misfire ON automation_scheduler_state(last_misfire_at)`<br>`sqlite_autoindex_automation_scheduler_state_1(job_id)` | 6 | none |
| `automation_suggestions` | `CREATE UNIQUE INDEX automation_suggestions_profile_workspace_dedup_key ON automation_suggestions(profile_id, workspace_id, dedup_key)`<br>`CREATE INDEX idx_automation_suggestions_profile_workspace_status ON automation_suggestions(profile_id, workspace_id, status, created_at, id)`<br>`sqlite_autoindex_automation_suggestions_1(id)` | 9 | `workspace_id → workspaces` |
| `automation_trigger_catalog_entries` | `CREATE INDEX idx_automation_trigger_catalog_workspace_order ON automation_trigger_catalog_entries(workspace_id, source_rank, name, trigger_id)`<br>`CREATE INDEX idx_automation_trigger_catalog_order ON automation_trigger_catalog_entries(source_rank, name, trigger_id)`<br>`sqlite_autoindex_automation_trigger_catalog_entries_1(trigger_id)` | 1 | none |
| `automation_trigger_catalog_filter_terms` | `sqlite_autoindex_automation_trigger_catalog_filter_terms_1(trigger_id,value)` | 2 | none |
| `automation_trigger_overlays` | `sqlite_autoindex_automation_trigger_overlays_1(trigger_id)` | 5 | none |
| `automation_triggers` | `CREATE UNIQUE INDEX uq_automation_triggers_workspace_name ON automation_triggers(workspace_id, name) WHERE scope = 'workspace'`<br>`CREATE UNIQUE INDEX uq_automation_triggers_webhook_id ON automation_triggers(webhook_id) WHERE webhook_id IS NOT NULL`<br>`CREATE UNIQUE INDEX uq_automation_triggers_global_name ON automation_triggers(name) WHERE scope = 'global'`<br>`CREATE INDEX idx_automation_triggers_loop_target ON automation_triggers(loop_name, loop_workspace_id) WHERE target_kind = 'loop'`<br>`CREATE INDEX idx_automation_triggers_profile ON automation_triggers(profile_id, id)`<br>`CREATE INDEX idx_automation_triggers_event ON automation_triggers(event)`<br>`CREATE INDEX idx_automation_triggers_enabled ON automation_triggers(enabled)`<br>`sqlite_autoindex_automation_triggers_1(id)` | 10 | `loop_workspace_id → workspaces`<br>`workspace_id → workspaces` |
| `automation_watch_events` | `CREATE INDEX idx_automation_watch_events_workspace_status_seq ON automation_watch_events(workspace_id, status, seq)` | 0 | none |
| `cmd_palette_pins` | `CREATE INDEX idx_cmd_palette_pins_order ON cmd_palette_pins (workspace_id, profile_lens_id, pinned_at ASC, command_id)`<br>`sqlite_autoindex_cmd_palette_pins_1(workspace_id,profile_lens_id,command_id)` | 4 | none |
| `cmd_palette_query_hits` | `CREATE INDEX idx_cmd_palette_query_hits_lookup ON cmd_palette_query_hits (workspace_id, profile_lens_id, query, last_used_at DESC, command_id)`<br>`sqlite_autoindex_cmd_palette_query_hits_1(workspace_id,profile_lens_id,query,command_id)` | 6 | none |
| `cmd_palette_usage` | `CREATE INDEX idx_cmd_palette_usage_recents ON cmd_palette_usage (workspace_id, profile_lens_id, last_used_at DESC, command_id)`<br>`sqlite_autoindex_cmd_palette_usage_1(workspace_id,profile_lens_id,command_id)` | 5 | none |
| `config_apply_records` | `CREATE INDEX idx_config_apply_records_status ON config_apply_records(status, updated_at DESC)`<br>`CREATE INDEX idx_config_apply_records_generation ON config_apply_records(generation DESC, created_at DESC)`<br>`CREATE INDEX idx_config_apply_records_desired ON config_apply_records(desired_config_hash, created_at DESC)`<br>`CREATE INDEX idx_config_apply_records_actor ON config_apply_records(actor, created_at DESC)`<br>`CREATE INDEX idx_config_apply_records_active ON config_apply_records(active_config_hash, created_at DESC)`<br>`sqlite_autoindex_config_apply_records_1(id)` | 0 | none |
| `dead_entities` | `sqlite_autoindex_dead_entities_1(profile_id,workspace_id,kind,entity_id)` | 4 | none |
| `event_summaries` | `CREATE INDEX idx_summaries_worktree ON event_summaries(worktree_id)`<br>`CREATE INDEX idx_summaries_workspace ON event_summaries(workspace_id)`<br>`CREATE INDEX idx_summaries_workflow ON event_summaries(workflow_id)`<br>`CREATE INDEX idx_summaries_type ON event_summaries(type)`<br>`CREATE INDEX idx_summaries_timestamp ON event_summaries(timestamp)`<br>`CREATE INDEX idx_summaries_task ON event_summaries(task_id)`<br>`CREATE INDEX idx_summaries_session ON event_summaries(session_id)`<br>`CREATE INDEX idx_summaries_run ON event_summaries(run_id)`<br>`CREATE INDEX idx_summaries_root ON event_summaries(root_session_id)`<br>`CREATE INDEX idx_summaries_provider_timestamp ON event_summaries(provider, timestamp DESC)`<br>`CREATE INDEX idx_summaries_parent ON event_summaries(parent_session_id)`<br>`CREATE INDEX idx_summaries_outcome_timestamp ON event_summaries(outcome, timestamp DESC)`<br>`CREATE INDEX idx_summaries_hook_event ON event_summaries(hook_event)`<br>`CREATE INDEX idx_summaries_actor ON event_summaries(actor_kind, actor_id)`<br>`sqlite_autoindex_event_summaries_1(id)` | 3 | `profile_id → profiles` |
| `extension_dev_links` | `sqlite_autoindex_extension_dev_links_1(extension_name,workspace_id)` | 0 | none |
| `extension_env_bindings` | `CREATE INDEX idx_extension_env_bindings_secret_ref ON extension_env_bindings (secret_ref)`<br>`sqlite_autoindex_extension_env_bindings_1(extension_name,profile_id,workspace_id,env_name)` | 7 | none |
| `extension_inputs` | `sqlite_autoindex_extension_inputs_1(extension,profile,workspace_id,input_id)` | 3 | none |
| `extension_installations` | `sqlite_autoindex_extension_installations_1(extension_name,profile_id,workspace_id)` | 0 | none |
| `extension_mcp_overrides` | `sqlite_autoindex_extension_mcp_overrides_2(profile,workspace_id,runtime_name)`<br>`sqlite_autoindex_extension_mcp_overrides_1(extension,profile,workspace_id,server)` | 7 | none |
| `extension_profile_enablement` | `sqlite_autoindex_extension_profile_enablement_1(extension_name,profile_id)` | 0 | `profile_id → profiles` |
| `extension_profile_markers` | `sqlite_autoindex_extension_profile_markers_1(extension_name,profile_name)` | 0 | none |
| `extensions` | `sqlite_autoindex_extensions_1(name)` | 1 | none |
| `gateway_device_sessions` | `CREATE UNIQUE INDEX gateway_device_sessions_token_hash ON gateway_device_sessions(token_hash)`<br>`sqlite_autoindex_gateway_device_sessions_1(id)` | 11 | none |
| `gateway_endpoint_state` | `sqlite_autoindex_gateway_endpoint_state_1(tier)` | 4 | none |
| `gateway_ingress_bindings` | `CREATE INDEX gateway_ingress_bindings_workspace ON gateway_ingress_bindings(workspace_id, subject_kind, subject_id) WHERE workspace_id IS NOT NULL`<br>`sqlite_autoindex_gateway_ingress_bindings_1(subject_kind,subject_id)` | 7 | none |
| `gateway_provider_activations` | `CREATE UNIQUE INDEX gateway_active_provider_per_tier ON gateway_provider_activations(tier) WHERE desired_state = 'enabled'`<br>`sqlite_autoindex_gateway_provider_activations_1(provider_name,tier)` | 5 | none |
| `gateway_providers` | `sqlite_autoindex_gateway_providers_1(name)` | 2 | none |
| `gateway_surface_exposure` | `sqlite_autoindex_gateway_surface_exposure_1(surface,tier)` | 5 | none |
| `loop_admission_claims` | `CREATE INDEX idx_loop_admission_claims_expiry ON loop_admission_claims(expires_at)`<br>`sqlite_autoindex_loop_admission_claims_1(workspace_id,loop_name,source_key,event_key)` | 2 | none |
| `loop_config` | `sqlite_autoindex_loop_config_1(workspace_id,loop_name)` | 4 | none |
| `loop_definition_snapshots` | `sqlite_autoindex_loop_definition_snapshots_1(workspace_id,definition_digest)` | 5 | none |
| `loop_effect_outbox` | `CREATE INDEX idx_loop_effect_outbox_pending ON loop_effect_outbox(state) WHERE state = 'pending'`<br>`sqlite_autoindex_loop_effect_outbox_1(loop_run_id,delivery_id)` | 4 | none |
| `loop_gate_decisions` | `CREATE INDEX idx_loop_gate_decisions_workspace_run ON loop_gate_decisions(workspace_id, loop_run_id, generation, gate_id)`<br>`sqlite_autoindex_loop_gate_decisions_1(loop_run_id,generation,gate_id,criterion_id)` | 2 | none |
| `loop_gate_verdicts` | `CREATE INDEX idx_loop_gate_verdicts_route_cause ON loop_gate_verdicts(loop_run_id, generation, route_cause_rank) WHERE route_cause_rank IS NOT NULL`<br>`sqlite_autoindex_loop_gate_verdicts_1(loop_run_id,generation,gate_id,item_index)` | 3 | none |
| `loop_generation_outputs` | `CREATE INDEX idx_loop_generation_outputs_output_ref ON loop_generation_outputs(output_ref)`<br>`CREATE INDEX idx_loop_generation_outputs_retry_due ON loop_generation_outputs(next_attempt_at, loop_run_id, generation, node_id, item_index) WHERE status = 'retrying' AND next_attempt_at IS NOT NULL`<br>`sqlite_autoindex_loop_generation_outputs_1(loop_run_id,generation,node_id,item_index)` | 17 | none |
| `loop_generations` | `sqlite_autoindex_loop_generations_1(loop_run_id,generation)` | 2 | none |
| `loop_goal_binding_retry_witnesses` | `sqlite_autoindex_loop_goal_binding_retry_witnesses_1(loop_run_id,handle,failed_binding_epoch)` | 2 | none |
| `loop_goal_checkpoints` | `sqlite_autoindex_loop_goal_checkpoints_1(loop_run_id,generation,node_id,item_index)` | 33 | none |
| `loop_goal_judge_attempts` | `sqlite_autoindex_loop_goal_judge_attempts_2(loop_run_id,generation,node_id,item_index,turn)`<br>`sqlite_autoindex_loop_goal_judge_attempts_1(attempt_id)` | 7 | none |
| `loop_goal_session_outbox` | `CREATE INDEX idx_loop_goal_session_outbox_pending ON loop_goal_session_outbox(id) WHERE delivered_at IS NULL`<br>`sqlite_autoindex_loop_goal_session_outbox_1(event_id)` | 4 | `loop_run_id → loop_runs` |
| `loop_goal_turns` | `sqlite_autoindex_loop_goal_turns_3(loop_run_id,prompt_id)`<br>`sqlite_autoindex_loop_goal_turns_2(loop_run_id,seq)`<br>`sqlite_autoindex_loop_goal_turns_1(loop_run_id,generation,node_id,item_index,turn)` | 12 | none |
| `loop_node_amendments` | `sqlite_autoindex_loop_node_amendments_1(loop_run_id,generation,node_id,item_index,amendment_seq)` | 2 | none |
| `loop_node_attempts` | `sqlite_autoindex_loop_node_attempts_1(loop_run_id,generation,node_id,item_index,attempt)` | 1 | none |
| `loop_node_controls` | `CREATE INDEX idx_loop_node_controls_attention ON loop_node_controls(attention_flag) WHERE attention_flag != ''`<br>`CREATE INDEX idx_loop_node_controls_quarantined ON loop_node_controls(quarantined) WHERE quarantined = 1`<br>`sqlite_autoindex_loop_node_controls_1(loop_run_id,node_id)` | 5 | none |
| `loop_node_lane_pauses` | `sqlite_autoindex_loop_node_lane_pauses_1(loop_run_id,generation,node_id,item_index)` | 0 | none |
| `loop_node_waits` | `CREATE INDEX idx_loop_node_waits_state ON loop_node_waits(claim_state)`<br>`CREATE INDEX idx_loop_node_waits_ladder ON loop_node_waits(next_escalation_at) WHERE claim_state = 'waiting' AND next_escalation_at IS NOT NULL`<br>`CREATE INDEX idx_loop_node_waits_due ON loop_node_waits(resume_at) WHERE claim_state = 'waiting' AND resume_at IS NOT NULL`<br>`sqlite_autoindex_loop_node_waits_1(loop_run_id,generation,node_id,item_index)` | 4 | none |
| `loop_output_blobs` | `sqlite_autoindex_loop_output_blobs_1(output_ref)` | 4 | none |
| `loop_requests` | `CREATE INDEX idx_loop_requests_pending ON loop_requests(workspace_id, state, expires_at, opened_at)`<br>`sqlite_autoindex_loop_requests_1(loop_run_id,generation,node_id,item_index)` | 1 | none |
| `loop_run_events` | `CREATE UNIQUE INDEX uq_loop_run_events_delivery ON loop_run_events(loop_run_id, delivery_key) WHERE delivery_key IS NOT NULL`<br>`CREATE INDEX idx_loop_run_events_watch_stream ON loop_run_events(workspace_id, watch_seq)`<br>`CREATE INDEX idx_loop_run_events_run_seq ON loop_run_events(loop_run_id, seq)`<br>`sqlite_autoindex_loop_run_events_1(id)` | 8 | none |
| `loop_runs` | `CREATE UNIQUE INDEX uq_loop_runs_active_session_goal ON loop_runs(origin_session_id) WHERE origin_kind='session' AND historical = 0 AND status IN ('queued','running','watching','needs-approval','paused')`<br>`CREATE INDEX idx_loop_runs_queue_order ON loop_runs(workspace_id, loop_name, status, created_at ASC, id ASC)`<br>`CREATE INDEX idx_loop_runs_catalog ON loop_runs(workspace_id, loop_name, created_at DESC, id DESC, status)`<br>`sqlite_autoindex_loop_runs_1(id)` | 69 | `profile_id → profiles` |
| `loop_session_bindings` | `CREATE UNIQUE INDEX uq_loop_session_bindings_active ON loop_session_bindings(loop_run_id, handle) WHERE state='active'`<br>`sqlite_autoindex_loop_session_bindings_2(loop_run_id,handle,binding_epoch)`<br>`sqlite_autoindex_loop_session_bindings_1(binding_attempt_id)` | 22 | none |
| `loop_session_cleanup` | `CREATE INDEX idx_loop_session_cleanup_pending ON loop_session_cleanup(id) WHERE completed_at IS NULL`<br>`sqlite_autoindex_loop_session_cleanup_2(loop_run_id,source_kind,source_id,source_epoch)`<br>`sqlite_autoindex_loop_session_cleanup_1(cleanup_id)` | 5 | none |
| `loop_timetravel_ops` | `CREATE UNIQUE INDEX uq_loop_timetravel_ops_idempotency ON loop_timetravel_ops(workspace_id, idempotency_key) WHERE idempotency_key != ''`<br>`sqlite_autoindex_loop_timetravel_ops_1(workspace_id,op_id)` | 0 | `source_run_id → loop_runs` |
| `loop_ui_annotations` | `sqlite_autoindex_loop_ui_annotations_1(workspace_id,loop_name,node_id)` | 3 | none |
| `marketplace_catalog_config` |  | 3 | none |
| `marketplace_catalog_entries` | `CREATE INDEX idx_marketplace_catalog_entries_source_name ON marketplace_catalog_entries(source, name, entry_id)`<br>`sqlite_autoindex_marketplace_catalog_entries_1(source,entry_id)` | 7 | none |
| `marketplace_catalog_state` | `sqlite_autoindex_marketplace_catalog_state_1(source)` | 9 | none |
| `mcp_auth_tokens` | `CREATE INDEX idx_mcp_auth_tokens_updated_at ON mcp_auth_tokens(updated_at)`<br>`sqlite_autoindex_mcp_auth_tokens_1(scope,workspace_id,owner,server_name)` | 7 | none |
| `mcp_oauth_registrations` | `sqlite_autoindex_mcp_oauth_registrations_1(scope,workspace_id,owner,server_name)` | 6 | none |
| `model_catalog_execution_contexts` | `sqlite_autoindex_model_catalog_execution_contexts_2(scope,profile_id,workspace_id,command_fingerprint)`<br>`sqlite_autoindex_model_catalog_execution_contexts_1(context_id)` | 1 | none |
| `model_catalog_option_values` | `CREATE INDEX idx_model_catalog_option_values_option ON model_catalog_option_values(context_id, source_id, provider_id, model_id, option_id, rank, value_id)`<br>`sqlite_autoindex_model_catalog_option_values_1(context_id,source_id,provider_id,model_id,option_id,value_id)` | 3 | none |
| `model_catalog_options` | `sqlite_autoindex_model_catalog_options_1(context_id,source_id,provider_id,model_id,option_id)` | 3 | none |
| `model_catalog_reasoning_efforts` | `sqlite_autoindex_model_catalog_reasoning_efforts_1(context_id,source_id,provider_id,model_id,effort)` | 2 | none |
| `model_catalog_rows` | `CREATE INDEX idx_model_catalog_rows_source_provider ON model_catalog_rows(context_id, source_id, provider_id)`<br>`CREATE INDEX idx_model_catalog_rows_provider_model ON model_catalog_rows(context_id, provider_id, model_id, priority DESC, refreshed_at DESC, source_id ASC)`<br>`sqlite_autoindex_model_catalog_rows_1(context_id,source_id,provider_id,model_id)` | 6 | none |
| `model_catalog_sources` | `CREATE INDEX idx_model_catalog_sources_provider ON model_catalog_sources(context_id, provider_id, refresh_state, stale)`<br>`sqlite_autoindex_model_catalog_sources_1(context_id,source_id,provider_id)` | 2 | none |
| `model_catalog_transport_binding_selections` | `sqlite_autoindex_model_catalog_transport_binding_selections_1(context_id,source_id,provider_id,model_id,transport_model_id,option_id)` | 3 | none |
| `model_catalog_transport_bindings` | `CREATE INDEX idx_model_catalog_transport_bindings_row ON model_catalog_transport_bindings(context_id, source_id, provider_id, model_id, rank, transport_model_id)`<br>`sqlite_autoindex_model_catalog_transport_bindings_1(context_id,source_id,provider_id,model_id,transport_model_id)` | 3 | none |
| `notification_cursors` | `CREATE INDEX notification_cursors_stream_sequence_idx ON notification_cursors(scope_kind, profile_id, workspace_id, stream_name, last_sequence DESC) WHERE last_sequence > 0`<br>`sqlite_autoindex_notification_cursors_1(scope_kind,profile_id,workspace_id,consumer_id,stream_name,subject_id)` | 5 | `profile_id → profiles` |
| `permission_log` | `CREATE INDEX idx_perm_session ON permission_log(session_id)`<br>`sqlite_autoindex_permission_log_1(id)` | 2 | none |
| `profile_credential_requirements` | `sqlite_autoindex_profile_credential_requirements_1(profile_id,provider,slot)` | 0 | none |
| `profile_lifecycle_op_credential_asks` | `sqlite_autoindex_profile_lifecycle_op_credential_asks_1(op_id,provider,slot)` | 0 | none |
| `profile_lifecycle_op_seed` | `sqlite_autoindex_profile_lifecycle_op_seed_1(op_id)` | 0 | none |
| `profile_lifecycle_op_steps` | `sqlite_autoindex_profile_lifecycle_op_steps_1(op_id,seq)` | 0 | none |
| `profile_lifecycle_ops` | `CREATE INDEX idx_profile_lifecycle_ops_profile_status ON profile_lifecycle_ops (profile_id, status, updated_at DESC)`<br>`sqlite_autoindex_profile_lifecycle_ops_1(id)` | 0 | none |
| `profile_selections` | `sqlite_autoindex_profile_selections_1(lens,workspace_id)` | 0 | `profile_id → profiles` |
| `profiles` | `sqlite_autoindex_profiles_2(name)`<br>`sqlite_autoindex_profiles_1(id)` | 0 | none |
| `resource_records` | `CREATE INDEX idx_resource_source ON resource_records(source_kind, source_id, kind)`<br>`CREATE INDEX idx_resource_scope ON resource_records(scope_kind, scope_id, kind)`<br>`CREATE INDEX idx_resource_owner ON resource_records(owner_kind, owner_id, kind)`<br>`CREATE INDEX idx_resource_kind ON resource_records(kind)`<br>`sqlite_autoindex_resource_records_1(kind,id)` | 4 | none |
| `resource_source_state` | `sqlite_autoindex_resource_source_state_1(source_kind,source_id)` | 0 | none |
| `scheduler_pause` |  | 3 | none |
| `session_creation_profiles` | `sqlite_autoindex_session_creation_profiles_1(profile_ref)` | 2 | none |
| `session_derivations` | `CREATE INDEX idx_session_derivations_child ON session_derivations(child_session_id)`<br>`CREATE INDEX idx_session_derivations_source ON session_derivations(source_session_id)`<br>`sqlite_autoindex_session_derivations_1(workspace_id,idempotency_key)` | 3 | none |
| `session_health` | `CREATE INDEX idx_session_health_workspace_agent ON session_health(workspace_id, agent_name, health, updated_at DESC)`<br>`CREATE INDEX idx_session_health_wake ON session_health(workspace_id, agent_name, eligible_for_wake, active_prompt, attachable)`<br>`sqlite_autoindex_session_health_1(session_id)` | 4 | none |
| `session_input_clear_traces` | `CREATE INDEX idx_session_input_clear_traces_pending ON session_input_clear_traces(session_id, created_at, entry_id) WHERE projected_at IS NULL`<br>`sqlite_autoindex_session_input_clear_traces_1(entry_id)` | 3 | `session_id → sessions` |
| `session_input_queue` | `CREATE UNIQUE INDEX uq_session_input_queue_synthetic_message ON session_input_queue(session_id, message_id) WHERE owner_kind = 'synthetic' AND message_id <> '' AND status IN ('queued','dispatching')`<br>`CREATE UNIQUE INDEX uq_session_input_queue_goal_prompt ON session_input_queue(loop_run_id, prompt_id) WHERE prompt_id IS NOT NULL`<br>`CREATE UNIQUE INDEX uq_session_input_queue_active_steer ON session_input_queue(session_id) WHERE mode = 'steer' AND status = 'queued'`<br>`CREATE UNIQUE INDEX uq_session_input_queue_prompt_admission ON session_input_queue(prompt_admission_id) WHERE prompt_admission_id IS NOT NULL`<br>`CREATE INDEX idx_session_input_queue_pending ON session_input_queue(session_id, status, delivery DESC, priority DESC, enqueued_at ASC, id ASC)`<br>`CREATE INDEX idx_session_input_queue_goal_owner ON session_input_queue(loop_run_id, task_run_id, owner_epoch, status, dispatchable, fence_kind)`<br>`CREATE INDEX idx_session_input_queue_generation ON session_input_queue(session_id, session_generation, status)`<br>`sqlite_autoindex_session_input_queue_1(id)` | 35 | none |
| `session_pending_interactions` | `CREATE UNIQUE INDEX uq_session_pending_interactions_active_provider_request ON session_pending_interactions(session_id, kind, provider_request_id) WHERE status IN ('pending', 'orphaned')`<br>`CREATE INDEX idx_session_pending_interactions_session_status ON session_pending_interactions(session_id, status, created_at, interaction_id)`<br>`sqlite_autoindex_session_pending_interactions_1(interaction_id)` | 0 | none |
| `session_prompt_admissions` | `CREATE INDEX idx_session_prompt_admissions_state ON session_prompt_admissions(workspace_id, session_id, state, updated_at)`<br>`sqlite_autoindex_session_prompt_admissions_3(workspace_id,session_id,message_id)`<br>`sqlite_autoindex_session_prompt_admissions_2(workspace_id,session_id,idempotency_key)`<br>`sqlite_autoindex_session_prompt_admissions_1(id)` | 11 | none |
| `session_subagent_wakes` | `CREATE UNIQUE INDEX uq_session_subagent_wakes_open ON session_subagent_wakes(parent_session_id) WHERE state = 'open'`<br>`CREATE INDEX idx_session_subagent_wakes_parent_state ON session_subagent_wakes(parent_session_id,state)`<br>`sqlite_autoindex_session_subagent_wakes_1(wake_message_id)` | 14 | none |
| `session_subagents` | `CREATE INDEX idx_session_subagents_provider_tool ON session_subagents(parent_session_id,provider_tool_call_id) WHERE origin = 'provider_native'`<br>`CREATE INDEX idx_session_subagents_parent_delivery ON session_subagents(parent_session_id,delivery) WHERE delivery IN ('pending','claimed')`<br>`CREATE INDEX idx_session_subagents_workspace_status ON session_subagents(workspace_id,status)`<br>`CREATE INDEX idx_session_subagents_parent_created ON session_subagents(parent_session_id,created_at)`<br>`sqlite_autoindex_session_subagents_3(parent_session_id,idempotency_key)`<br>`sqlite_autoindex_session_subagents_2(child_session_id)`<br>`sqlite_autoindex_session_subagents_1(id)` | 28 | `wake_message_id → session_subagent_wakes` |
| `sessions` | `CREATE INDEX idx_sessions_worktree ON sessions(worktree_id) WHERE worktree_id IS NOT NULL`<br>`CREATE INDEX idx_sessions_type_depth ON sessions(session_type, spawn_depth)`<br>`CREATE INDEX idx_sessions_spawn_role ON sessions(spawn_role)`<br>`CREATE INDEX idx_sessions_soul_snapshot ON sessions(soul_snapshot_id)`<br>`CREATE INDEX idx_sessions_root ON sessions(root_session_id)`<br>`CREATE INDEX idx_sessions_resumable ON sessions(state, failure_kind, last_update_at, updated_at)`<br>`CREATE INDEX idx_sessions_parent ON sessions(parent_session_id)`<br>`CREATE INDEX idx_sessions_profile_catalog_archive_recent ON sessions( profile_id, workspace_id, archived_at, state, updated_at DESC, created_at DESC, id DESC )`<br>`CREATE INDEX idx_sessions_catalog_archive_recent ON sessions( workspace_id, archived_at, state, updated_at DESC, created_at DESC, id DESC )`<br>`CREATE INDEX idx_sessions_profile_catalog_created ON sessions(profile_id, workspace_id, archived_at, created_at DESC, id DESC)`<br>`CREATE INDEX idx_sessions_catalog_created ON sessions(workspace_id, archived_at, created_at DESC, id DESC)`<br>`CREATE INDEX idx_sessions_profile_workspace_catalog_navigator ON sessions(profile_id, workspace_id, archived_at, (CASE WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0 WHEN pending_clarify_count > 0 THEN 0 WHEN state = 'stopped' THEN 3 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC)`<br>`CREATE INDEX idx_sessions_workspace_catalog_navigator ON sessions(workspace_id, archived_at, (CASE WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0 WHEN pending_clarify_count > 0 THEN 0 WHEN state = 'stopped' THEN 3 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC)`<br>`CREATE INDEX idx_sessions_profile_global_catalog_navigator ON sessions(profile_id, archived_at, (CASE WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0 WHEN pending_clarify_count > 0 THEN 0 WHEN state = 'stopped' THEN 3 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC)`<br>`CREATE INDEX idx_sessions_global_catalog_navigator ON sessions(archived_at, (CASE WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0 WHEN pending_clarify_count > 0 THEN 0 WHEN state = 'stopped' THEN 3 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC)`<br>`CREATE INDEX idx_sessions_profile_catalog_recent ON sessions(profile_id, workspace_id, state, updated_at DESC, created_at DESC, id DESC)`<br>`CREATE INDEX idx_sessions_catalog_recent ON sessions(workspace_id, state, updated_at DESC, created_at DESC, id DESC)`<br>`CREATE INDEX idx_sessions_profile_catalog_activity ON sessions( profile_id, workspace_id, state, COALESCE(last_update_at, updated_at) DESC, updated_at DESC, created_at DESC, id DESC )`<br>`CREATE INDEX idx_sessions_catalog_activity ON sessions( workspace_id, state, COALESCE(last_update_at, updated_at) DESC, updated_at DESC, created_at DESC, id DESC )`<br>`CREATE INDEX idx_sessions_global_catalog_attention ON sessions(archived_at, (CASE WHEN (stop_verification_failed = 1 AND state <> 'stopped') OR pending_permission_count > 0 OR pending_clarify_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') OR (state = 'stopped' AND trim(COALESCE(failure_kind, '')) <> '' AND trim(COALESCE(failure_kind, '')) <> 'cancellation') THEN 0 WHEN state = 'active' AND trim(COALESCE(stall_state, '')) <> 'stalled' AND trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) = '' AND last_settled_revision > last_seen_revision THEN 1 ELSE 2 END), COALESCE(attention_changed_at, updated_at) DESC, updated_at DESC, created_at DESC, id DESC)`<br>`CREATE INDEX idx_sessions_global_catalog_activity ON sessions(archived_at, COALESCE(last_update_at, updated_at) DESC, updated_at DESC, created_at DESC, id DESC)`<br>`CREATE INDEX idx_sessions_global_catalog_recent ON sessions(archived_at, updated_at DESC, created_at DESC, id DESC)`<br>`CREATE INDEX idx_sessions_attach_lock ON sessions(attached_to, attach_expires_at)`<br>`sqlite_autoindex_sessions_2(workspace_id,id)`<br>`sqlite_autoindex_sessions_1(id)` | 31 | none |
| `skill_exposures` | `CREATE INDEX idx_skill_exposures_workspace_id ON skill_exposures(workspace_id)`<br>`CREATE UNIQUE INDEX idx_skill_exposures_owner_target ON skill_exposures(skill_name, owner_scope, COALESCE(workspace_id, ''), target_slug)`<br>`sqlite_autoindex_skill_exposures_1(link_path)` | 5 | none |
| `task_block_recurrences` | `sqlite_autoindex_task_block_recurrences_1(task_id,kind)` | 4 | none |
| `task_blocks` | `CREATE INDEX idx_task_blocks_open ON task_blocks(task_id) WHERE cleared_at IS NULL`<br>`CREATE INDEX idx_task_blocks_expiry ON task_blocks(expires_at) WHERE cleared_at IS NULL AND expires_at IS NOT NULL`<br>`sqlite_autoindex_task_blocks_1(id)` | 10 | `task_id → tasks` |
| `task_dependencies` | `CREATE INDEX idx_task_dependencies_task ON task_dependencies(task_id, created_at ASC, depends_on_task_id ASC)`<br>`CREATE INDEX idx_task_dependencies_depends_on ON task_dependencies(depends_on_task_id, task_id ASC)`<br>`sqlite_autoindex_task_dependencies_1(task_id,depends_on_task_id,kind)` | 6 | none |
| `task_designation_rollups` | `sqlite_autoindex_task_designation_rollups_1(designation_group_id)` | 1 | `task_id → tasks` |
| `task_events` | `CREATE UNIQUE INDEX uq_task_events_event_seq ON task_events(event_seq)`<br>`CREATE INDEX idx_task_events_wake_event ON task_events(task_id, event_type, json_extract(payload_json, '$.wake_event_id')) WHERE event_type IN ('task.wake.delivered', 'task.wake.suppressed')`<br>`CREATE INDEX idx_task_events_type_seq ON task_events(event_type, event_seq)`<br>`CREATE INDEX idx_task_events_type ON task_events(event_type, timestamp DESC, id DESC)`<br>`CREATE INDEX idx_task_events_task_seq ON task_events(task_id, event_seq ASC)`<br>`CREATE INDEX idx_task_events_task ON task_events(task_id, timestamp DESC, id DESC)`<br>`CREATE INDEX idx_task_events_run ON task_events(run_id, timestamp DESC, id DESC)`<br>`sqlite_autoindex_task_events_1(id)` | 8 | none |
| `task_execution_profiles` | `CREATE INDEX task_execution_profiles_task_id_idx ON task_execution_profiles(task_id)`<br>`sqlite_autoindex_task_execution_profiles_1(task_id)` | 3 | none |
| `task_profile_agents` | `CREATE INDEX task_profile_agents_lookup_idx ON task_profile_agents(role, preference, agent_name, task_id)`<br>`sqlite_autoindex_task_profile_agents_1(task_id,role,preference,agent_name)` | 3 | none |
| `task_profile_capabilities` | `CREATE INDEX task_profile_capabilities_lookup_idx ON task_profile_capabilities(role, preference, capability_id, task_id)`<br>`sqlite_autoindex_task_profile_capabilities_1(task_id,role,preference,capability_id)` | 3 | none |
| `task_run_idempotency` | `CREATE INDEX idx_task_run_idempotency_run ON task_run_idempotency(run_id)`<br>`sqlite_autoindex_task_run_idempotency_1(idempotency_key,origin_kind,origin_ref)` | 3 | none |
| `task_run_preferred_capabilities` | `CREATE INDEX idx_task_run_preferred_capabilities_capability ON task_run_preferred_capabilities(capability_id, run_id)`<br>`sqlite_autoindex_task_run_preferred_capabilities_1(run_id,capability_id)` | 3 | none |
| `task_run_required_capabilities` | `CREATE INDEX idx_task_run_required_capabilities_capability ON task_run_required_capabilities(capability_id, run_id)`<br>`sqlite_autoindex_task_run_required_capabilities_1(run_id,capability_id)` | 3 | none |
| `task_run_reviews` | `CREATE UNIQUE INDEX uq_task_run_reviews_run_round_attempt ON task_run_reviews(run_id, review_round, attempt)`<br>`CREATE UNIQUE INDEX uq_task_run_reviews_reviewer_session_active ON task_run_reviews(reviewer_session_id) WHERE reviewer_session_id IS NOT NULL AND status IN ('routed', 'in_review')`<br>`CREATE UNIQUE INDEX uq_task_run_reviews_delivery ON task_run_reviews(review_id, delivery_id) WHERE delivery_id IS NOT NULL`<br>`CREATE INDEX idx_task_run_reviews_task_round_attempt ON task_run_reviews(task_id, review_round, attempt)`<br>`CREATE INDEX idx_task_run_reviews_run_status ON task_run_reviews(run_id, status)`<br>`CREATE INDEX idx_task_run_reviews_reviewer_session ON task_run_reviews(reviewer_session_id, status)`<br>`CREATE INDEX idx_task_run_reviews_reviewer_agent ON task_run_reviews(reviewer_agent_name, status)`<br>`CREATE INDEX idx_task_run_reviews_deadline ON task_run_reviews(status, deadline_at)`<br>`sqlite_autoindex_task_run_reviews_1(review_id)` | 7 | `parent_review_id → task_run_reviews` |
| `task_run_starvation` | `CREATE INDEX idx_task_run_starvation_tier ON task_run_starvation(escalation_tier, run_id)`<br>`sqlite_autoindex_task_run_starvation_1(run_id)` | 4 | none |
| `task_run_terminal_commands` | `CREATE INDEX idx_task_run_terminal_commands_phase ON task_run_terminal_commands(phase, admitted_at, run_id)`<br>`sqlite_autoindex_task_run_terminal_commands_2(run_id)`<br>`sqlite_autoindex_task_run_terminal_commands_1(command_id)` | 6 | `task_id → tasks` |
| `task_runs` | `CREATE INDEX idx_task_runs_coordinator_provenance ON task_runs(task_id, workspace_id, loop_run_id) WHERE run_kind = 'coordinator'`<br>`CREATE INDEX idx_task_runs_loop_status ON task_runs(loop_run_id, status, task_id) WHERE loop_run_id IS NOT NULL`<br>`CREATE UNIQUE INDEX uq_task_runs_review_id ON task_runs(review_id) WHERE review_id IS NOT NULL`<br>`CREATE UNIQUE INDEX uq_task_runs_active_loop_coordinator ON task_runs(loop_run_id) WHERE run_kind = 'coordinator' AND status IN ('queued', 'claimed', 'starting', 'running')`<br>`CREATE INDEX idx_task_runs_workspace_active ON task_runs(workspace_id, status, lease_until) WHERE workspace_id IS NOT NULL AND run_kind IN ('worker', 'coordinator')`<br>`CREATE INDEX idx_task_runs_task_status ON task_runs(task_id, status, queued_at DESC, id DESC)`<br>`CREATE INDEX idx_task_runs_task_review_round ON task_runs(task_id, review_round) WHERE review_round > 0`<br>`CREATE INDEX idx_task_runs_task ON task_runs(task_id, queued_at DESC, id DESC)`<br>`CREATE INDEX idx_task_runs_status ON task_runs(status)`<br>`CREATE INDEX idx_task_runs_worktree ON task_runs(worktree_id) WHERE worktree_id IS NOT NULL`<br>`CREATE INDEX idx_task_runs_session_status ON task_runs(session_id, status, lease_until)`<br>`CREATE INDEX idx_task_runs_session ON task_runs(session_id)`<br>`CREATE INDEX idx_task_runs_review_request ON task_runs(review_request_id) WHERE review_request_id IS NOT NULL`<br>`CREATE INDEX idx_task_runs_previous ON task_runs(previous_run_id)`<br>`CREATE INDEX idx_task_runs_pending_claim ON task_runs(status, lease_until, queued_at, id)`<br>`CREATE INDEX idx_task_runs_parent_run ON task_runs(parent_run_id)`<br>`CREATE INDEX idx_task_runs_designation_group ON task_runs(task_id, designation_group_id)`<br>`CREATE INDEX idx_task_runs_active_lease_recovery ON task_runs(status, lease_until, heartbeat_at, id)`<br>`sqlite_autoindex_task_runs_2(workspace_id,id)`<br>`sqlite_autoindex_task_runs_1(id)` | 59 | none |
| `task_triage_state` | `CREATE INDEX idx_task_triage_task ON task_triage_state(task_id, updated_at DESC)`<br>`CREATE INDEX idx_task_triage_actor ON task_triage_state(actor_kind, actor_id, updated_at DESC, task_id)`<br>`sqlite_autoindex_task_triage_state_1(task_id,actor_kind,actor_id)` | 3 | none |
| `tasks` | `CREATE INDEX idx_tasks_unsettled ON tasks(id) WHERE status NOT IN ('completed','failed','canceled')`<br>`CREATE INDEX idx_tasks_workspace ON tasks(workspace_id)`<br>`CREATE INDEX idx_tasks_status ON tasks(status)`<br>`CREATE INDEX idx_tasks_scope ON tasks(scope)`<br>`CREATE INDEX idx_tasks_review_round ON tasks(review_round)`<br>`CREATE INDEX idx_tasks_review_policy ON tasks(review_policy)`<br>`CREATE INDEX idx_tasks_priority ON tasks(priority)`<br>`CREATE INDEX idx_tasks_paused ON tasks(paused, updated_at DESC)`<br>`CREATE INDEX idx_tasks_parent ON tasks(parent_task_id)`<br>`CREATE INDEX idx_tasks_owner ON tasks(owner_kind, owner_ref)`<br>`CREATE INDEX idx_tasks_current_run ON tasks(current_run_id)`<br>`CREATE INDEX idx_tasks_created_by ON tasks(created_by_kind, created_by_ref)`<br>`CREATE INDEX idx_tasks_approval_state ON tasks(approval_state)`<br>`sqlite_autoindex_tasks_1(id)` | 24 | `profile_id → profiles` |
| `token_stats` | `CREATE UNIQUE INDEX idx_token_stats_session_agent ON token_stats(session_id, agent_name)`<br>`CREATE INDEX idx_token_stats_session ON token_stats(session_id)`<br>`sqlite_autoindex_token_stats_1(id)` | 2 | none |
| `token_usage_daily` | `CREATE INDEX idx_token_usage_daily_workspace ON token_usage_daily(workspace_id, day)`<br>`CREATE INDEX idx_token_usage_daily_profile_day ON token_usage_daily (profile_id, day)`<br>`sqlite_autoindex_token_usage_daily_1(day,profile_id,workspace_id,agent_name)` | 2 | none |
| `tool_approval_grants` | `CREATE INDEX idx_tool_approval_grants_list ON tool_approval_grants (profile_id, workspace_id, created_at DESC, id)`<br>`CREATE INDEX idx_tool_approval_grants_lookup ON tool_approval_grants (profile_id, workspace_id, tool_id, agent_name, input_digest)`<br>`sqlite_autoindex_tool_approval_grants_2(profile_id,workspace_id,agent_name,tool_id,input_digest)`<br>`sqlite_autoindex_tool_approval_grants_1(id)` | 4 | none |
| `tool_approval_pending` | `CREATE INDEX idx_tool_approval_pending_recovery ON tool_approval_pending (approval_status, execution_status, resume_fence, expires_at)`<br>`CREATE INDEX idx_tool_approval_pending_workspace_status ON tool_approval_pending (workspace_id, approval_status, expires_at, approval_id)`<br>`sqlite_autoindex_tool_approval_pending_2(invocation_id)`<br>`sqlite_autoindex_tool_approval_pending_1(approval_id)` | 7 | `profile_id → profiles` |
| `tool_processes` | `CREATE INDEX idx_tool_processes_tool_call ON tool_processes(tool_call_id)`<br>`CREATE INDEX idx_tool_processes_terminal ON tool_processes(terminal_id)`<br>`CREATE INDEX idx_tool_processes_state_updated ON tool_processes(state, updated_at)`<br>`CREATE INDEX idx_tool_processes_session_turn ON tool_processes(session_id, turn_id)`<br>`CREATE INDEX idx_tool_processes_hook ON tool_processes(hook_name)`<br>`CREATE INDEX idx_tool_processes_extension ON tool_processes(extension_name)`<br>`sqlite_autoindex_tool_processes_1(id)` | 2 | none |
| `vault_secrets` | `CREATE INDEX idx_vault_secrets_updated_at ON vault_secrets(updated_at)`<br>`CREATE INDEX idx_vault_secrets_kind ON vault_secrets(kind)`<br>`sqlite_autoindex_vault_secrets_1(ref)` | 3 | none |
| `workspace_deletion_intents` | `CREATE INDEX idx_workspace_deletion_intents_requested ON workspace_deletion_intents(requested_at, workspace_id)`<br>`sqlite_autoindex_workspace_deletion_intents_1(workspace_id)` | 4 | none |
| `workspaces` | `CREATE INDEX idx_workspaces_name ON workspaces(name)`<br>`sqlite_autoindex_workspaces_3(name)`<br>`sqlite_autoindex_workspaces_2(root_dir)`<br>`sqlite_autoindex_workspaces_1(id)` | 9 | none |
| `worktree_exit_ops` | `CREATE UNIQUE INDEX idx_worktree_exit_ops_active ON worktree_exit_ops(worktree_id) WHERE state = 'running'`<br>`sqlite_autoindex_worktree_exit_ops_1(op_id)` | 5 | `workspace_id,worktree_id → worktrees` |
| `worktree_forge_status` | `sqlite_autoindex_worktree_forge_status_1(worktree_id)` | 2 | none |
| `worktree_status` | `sqlite_autoindex_worktree_status_1(worktree_id)` | 2 | none |
| `worktrees` | `CREATE UNIQUE INDEX idx_worktrees_live_path ON worktrees(path) WHERE state IN ('pending', 'ready', 'removing')`<br>`CREATE UNIQUE INDEX idx_worktrees_reserved_name ON worktrees(workspace_id, name) WHERE state <> 'dismissed'`<br>`CREATE INDEX idx_worktrees_workspace_state ON worktrees(workspace_id, state)`<br>`sqlite_autoindex_worktrees_2(workspace_id,id)`<br>`sqlite_autoindex_worktrees_1(id)` | 17 | `profile_id → profiles` |

### Global static-query scan/sort triage

This is a coverage queue, not a list of defects. Whole-table catalogs, small state tables, JSON argument rowsets, filtered sorts, and recursive CTE scans can be appropriate. Empty-string parameters were used only to compile plans; no timings are inferred from these plans.

| Query | Scan/sort plan detail |
| --- | --- |
| `HydrateAutomationJobCatalog` | `USE TEMP B-TREE FOR ORDER BY` |
| `HydrateAutomationTriggerCatalog` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListAutomationSuggestions` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListAutomationSuggestionsAllProfiles` | `SCAN automation_suggestions`; `USE TEMP B-TREE FOR ORDER BY` |
| `ListAcceptedAutomationSuggestionsAllProfiles` | `SCAN automation_suggestions`; `USE TEMP B-TREE FOR ORDER BY` |
| `ListDeadEntities` | `SCAN dead_entities`; `USE TEMP B-TREE FOR ORDER BY` |
| `DeleteExtensionEnvBindingsByWorkspace` | `SCAN extension_env_bindings` |
| `ListExtensionMCPOverrides` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListAllExtensionMCPOverrides` | `USE TEMP B-TREE FOR LAST 2 TERMS OF ORDER BY` |
| `ListGatewayProviderActivations` | `SCAN gateway_provider_activations`; `USE TEMP B-TREE FOR ORDER BY` |
| `ListGatewaySurfaceExposure` | `SCAN gateway_surface_exposure`; `USE TEMP B-TREE FOR ORDER BY` |
| `DisableAllGatewayProviderActivations` | `SCAN gateway_provider_activations` |
| `DisableAllGatewaySurfaceExposure` | `SCAN gateway_surface_exposure` |
| `ListGatewayDevices` | `SCAN gateway_device_sessions`; `USE TEMP B-TREE FOR ORDER BY` |
| `CountActiveGatewayDeviceSessions` | `SCAN gateway_device_sessions` |
| `ListActiveGatewayDeviceSessions` | `SCAN gateway_device_sessions`; `USE TEMP B-TREE FOR ORDER BY` |
| `GetGatewayWebhookIngressSubject` | `USE TEMP B-TREE FOR ORDER BY` |
| `InsertWorkspace` | `SCAN automation_triggers`; `SCAN automation_triggers`; `SCAN automation_jobs`; `SCAN automation_jobs`; `SCAN attention_workspace_mutes` |
| `DeleteSessionsByWorkspace` | `SCAN session_input_clear_traces`; `SCAN agent_heartbeat_wake_events` |
| `DeleteSession` | `SCAN session_input_clear_traces`; `SCAN agent_heartbeat_wake_events` |
| `DeleteWorkspace` | `SCAN automation_triggers`; `SCAN automation_triggers`; `SCAN automation_jobs`; `SCAN automation_jobs`; `SCAN attention_workspace_mutes` |
| `ReadGoalSessionProjection` | `USE TEMP B-TREE FOR ORDER BY` |
| `ReadLatestGoalSessionCheckpoint` | `USE TEMP B-TREE FOR LAST 3 TERMS OF ORDER BY` |
| `ReconcileLoopSessionCleanup` | `SCAN loop_session_bindings` |
| `FindGoalReportTargets` | `USE TEMP B-TREE FOR ORDER BY` |
| `ResolveActiveGoalOriginAliases` | `USE TEMP B-TREE FOR ORDER BY` |
| `InsertHeartbeatSnapshot` | `SCAN agent_heartbeat_wake_state`; `SCAN agent_heartbeat_wake_events`; `SCAN agent_heartbeat_revisions` |
| `MarkSessionHealthStale` | `SCAN session_health` |
| `SweepHeartbeatWakeEvents` | `USE TEMP B-TREE FOR LAST TERM OF ORDER BY` |
| `InsertLoopRun` | `SCAN loop_goal_session_outbox`; `SCAN loop_timetravel_ops` |
| `FindNewestSessionGoal` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListWaitingLoopNodeInventory` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListQuarantinedLoopNodeInventory` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListAttentionLoopNodeInventory` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListRetryingLoopNodeInventory` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListLoopEffectOutbox` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListPendingLoopEffects` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListSessionLoopWork` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListExpirylessLoopEventWaits` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListLoopReconciliationCandidates` | `USE TEMP B-TREE FOR ORDER BY`; `USE TEMP B-TREE FOR ORDER BY`; `SCAN candidate` |
| `ListLoopProvenance` | `USE TEMP B-TREE FOR LAST 2 TERMS OF ORDER BY`; `SCAN tr` |
| `ListLoopCatalogAggregates` | `USE TEMP B-TREE FOR DISTINCT`; `SCAN requested` |
| `ListLoopCatalogLatestRuns` | `USE TEMP B-TREE FOR DISTINCT`; `SCAN requested` |
| `ListLoopRunsMissingActiveCoordinator` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListQueuedLoopRunsReadyForPromotion` | `USE TEMP B-TREE FOR ORDER BY` |
| `GetLastCoordinatorTaskIDForLoopRun` | `USE TEMP B-TREE FOR ORDER BY` |
| `GetLastCoordinatorRunIDForLoopRun` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListAvailableLoopOutputRefs` | `USE TEMP B-TREE FOR DISTINCT` |
| `SweepOrphanedLoopOutputBlobs` | `SCAN loop_output_blobs`; `SCAN loop_goal_turns`; `SCAN loop_goal_judge_attempts`; `SCAN loop_goal_checkpoints`; `SCAN loop_requests`; `SCAN loop_node_amendments` |
| `DeleteUnconfiguredMarketplaceEntries` | `SCAN marketplace_catalog_entries` |
| `DeleteUnconfiguredMarketplaceStates` | `SCAN marketplace_catalog_state` |
| `DeleteModelCatalogSource` | `SCAN model_catalog_sources` |
| `DeleteTokenStatsBefore` | `SCAN token_stats` |
| `DeletePermissionLogsBefore` | `SCAN permission_log` |
| `SweepExpiredSessionAttachLocks` | `SCAN sessions` |
| `UpsertSession` | `SCAN session_input_clear_traces`; `SCAN agent_heartbeat_wake_events` |
| `ListPendingSessionInputs` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListSessionInputQueueSummary` | `USE TEMP B-TREE FOR GROUP BY` |
| `ListSubagents` | `SCAN session_subagents`; `USE TEMP B-TREE FOR ORDER BY` |
| `InsertSubagentWake` | `SCAN session_subagents` |
| `ListSubagentWakeRows` | `SCAN session_subagents`; `USE TEMP B-TREE FOR ORDER BY` |
| `SettleSubagentWakeRows` | `SCAN session_subagents` |
| `CancelEmptySubagentWake` | `SCAN session_subagents` |
| `CancelEmptyParentSubagentWakes` | `SCAN session_subagents` |
| `ListStaleReservedSubagents` | `SCAN session_subagents`; `USE TEMP B-TREE FOR ORDER BY` |
| `ListUnfinalizedDelegatedSubagents` | `SCAN session_subagents`; `USE TEMP B-TREE FOR ORDER BY` |
| `ListOpenSubagentWakes` | `SCAN session_subagent_wakes`; `USE TEMP B-TREE FOR ORDER BY` |
| `ListPendingSubagents` | `SCAN session_subagents`; `USE TEMP B-TREE FOR ORDER BY` |
| `ListOrphanSubagentSessions` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListSubagentArchiveFamily` | `SCAN parent`; `SCAN family`; `USE TEMP B-TREE FOR ORDER BY` |
| `ListSubagentWakesByParent` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListUnfinalizedNativeSubagents` | `USE TEMP B-TREE FOR LAST TERM OF ORDER BY` |
| `ListSkillExposuresByOwner` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListSkillExposuresByCanonicalDir` | `SCAN skill_exposures`; `USE TEMP B-TREE FOR ORDER BY` |
| `ListTaskDependents` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListTaskEvents` | `SCAN task_events`; `USE TEMP B-TREE FOR ORDER BY` |
| `ListAllTaskBlocks` | `SCAN task_blocks`; `USE TEMP B-TREE FOR ORDER BY` |
| `ListOpenTaskBlocks` | `USE TEMP B-TREE FOR ORDER BY` |
| `HasClearedTaskBlockKind` | `SCAN task_blocks` |
| `ListExpiredTaskBlockCandidates` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListExpiredTaskBlockIDs` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListAutonomyLeaseHandles` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListExpiredTaskRunLeaseIDs` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListLiveLoopTaskRunIDs` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListOpenTaskDescendantIDs` | `SCAN parent` |
| `InsertTask` | `SCAN task_run_terminal_commands`; `SCAN task_designation_rollups`; `SCAN task_blocks` |
| `DeleteTask` | `SCAN task_run_terminal_commands`; `SCAN task_designation_rollups`; `SCAN task_blocks` |
| `ListTaskRunsByStatus` | `USE TEMP B-TREE FOR ORDER BY` |
| `GetRetryChildRunID` | `USE TEMP B-TREE FOR ORDER BY` |
| `LookupRunReviewBySession` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListRunReviews` | `SCAN task_run_reviews`; `USE TEMP B-TREE FOR ORDER BY` |
| `ListRunStarvation` | `SCAN task_run_starvation` |
| `FindActiveDesignationProjectionCandidate` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListTaskRunTerminalCommands` | `SCAN task_run_terminal_commands`; `USE TEMP B-TREE FOR ORDER BY` |
| `LookupApprovalGrant` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListPendingToolApprovals` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListMCPAuthTokenRefsByWorkspace` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListMCPOAuthRegistrationRefsByWorkspace` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListParkedWatchEventSubscriptions` | `USE TEMP B-TREE FOR LAST 2 TERMS OF ORDER BY` |
| `ListParkedWatchEventSubscriptionsPage` | `USE TEMP B-TREE FOR LAST 2 TERMS OF ORDER BY` |
| `InsertWorktree` | `SCAN worktree_exit_ops` |
| `ListWorktrees` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListRecoverableWorktrees` | `USE TEMP B-TREE FOR LAST 2 TERMS OF ORDER BY` |
| `DeletePendingWorktree` | `SCAN worktree_exit_ops` |
| `DeleteRunMaterialization` | `SCAN worktree_exit_ops` |
| `ListRunningWorktreeExitOperations` | `USE TEMP B-TREE FOR ORDER BY` |

## Session table inventory

Each table lists every explicit and automatic index, static query reference count (dynamic repositories are reviewed separately), and foreign-key probes that still scan. An index on either member of a composite child key can bound the probe and therefore is not classified as missing merely because the complete key is absent. Partial indexes qualify only when the FK equality implies their predicate.

| Table | Indexes (key columns; partial predicates retained) | Static query references | FK probes without index search |
| --- | --- | ---: | --- |
| `conversation_rewind_receipts` | `sqlite_autoindex_conversation_rewind_receipts_1(idempotency_key)` | 3 | none |
| `conversation_rewind_state` |  | 4 | none |
| `events` | `CREATE INDEX idx_events_type ON events(type)`<br>`CREATE INDEX idx_events_turn ON events(turn_id)`<br>`CREATE INDEX idx_events_transcript_entry_sequence ON events(transcript_entry_key, sequence)`<br>`CREATE INDEX idx_events_timestamp ON events(timestamp)`<br>`CREATE UNIQUE INDEX idx_events_sequence ON events(sequence)`<br>`sqlite_autoindex_events_1(id)` | 17 | none |
| `hook_runs` | `CREATE INDEX idx_hook_runs_recorded_at ON hook_runs(recorded_at)`<br>`CREATE INDEX idx_hook_runs_event ON hook_runs(event)`<br>`sqlite_autoindex_hook_runs_1(id)` | 2 | none |
| `session_db_identity` |  | 0 | none |
| `session_db_owner` |  | 0 | none |
| `token_usage` | `CREATE INDEX idx_usage_timestamp ON token_usage(timestamp)`<br>`sqlite_autoindex_token_usage_1(turn_id)` | 3 | none |
| `transcript_entries` | `CREATE INDEX idx_transcript_entries_visible_start ON transcript_entries(start_sequence) WHERE message_json IS NOT NULL`<br>`CREATE INDEX idx_transcript_entries_updated ON transcript_entries(updated_sequence, start_sequence)`<br>`CREATE INDEX idx_transcript_entries_role_sequence ON transcript_entries(kind, start_sequence)`<br>`CREATE INDEX idx_transcript_entries_turn ON transcript_entries(kind, turn_id, start_sequence)`<br>`CREATE UNIQUE INDEX idx_transcript_entries_message_id ON transcript_entries(message_id) WHERE message_id IS NOT NULL`<br>`sqlite_autoindex_transcript_entries_2(start_sequence)`<br>`sqlite_autoindex_transcript_entries_1(entry_key)` | 17 | none |
| `transcript_projection_state` |  | 4 | none |
| `transcript_tool_routes` | `sqlite_autoindex_transcript_tool_routes_1(tool_key)` | 4 | none |

### Session static-query scan/sort triage

This is a coverage queue, not a list of defects. Whole-table catalogs, small state tables, JSON argument rowsets, filtered sorts, and recursive CTE scans can be appropriate. Empty-string parameters were used only to compile plans; no timings are inferred from these plans.

| Query | Scan/sort plan detail |
| --- | --- |
| `ListTokenUsage` | `USE TEMP B-TREE FOR LAST TERM OF ORDER BY` |
| `CountTranscriptEntriesCrossingCut` | `SCAN bounds` |
| `DeleteTranscriptToolRoutesInRange` | `SCAN transcript_tool_routes` |
| `ListMissingUnarchivedCompactionEntries` | `USE TEMP B-TREE FOR ORDER BY` |
| `ListTranscriptTextEventsForUpgrade` | `USE TEMP B-TREE FOR ORDER BY` |

## Workspace table inventory

Each table lists every explicit and automatic index, static query reference count (dynamic repositories are reviewed separately), and foreign-key probes that still scan. An index on either member of a composite child key can bound the probe and therefore is not classified as missing merely because the complete key is absent. Partial indexes qualify only when the FK equality implies their predicate.

| Table | Indexes (key columns; partial predicates retained) | Static query references | FK probes without index search |
| --- | --- | ---: | --- |
| `terminal_artifacts` | `CREATE INDEX idx_terminal_artifacts_path ON terminal_artifacts (path, id)`<br>`CREATE INDEX idx_terminal_artifacts_expires ON terminal_artifacts (expires_at, id)`<br>`CREATE INDEX idx_terminal_artifacts_profile ON terminal_artifacts (profile_id, id)`<br>`CREATE INDEX idx_terminal_artifacts_command ON terminal_artifacts (command_id, id)`<br>`sqlite_autoindex_terminal_artifacts_1(id)` | 5 | none |
| `terminal_commands` | `CREATE INDEX idx_terminal_commands_recording ON terminal_commands (recording_id)`<br>`CREATE INDEX idx_terminal_commands_started ON terminal_commands (started_at DESC, id DESC)`<br>`CREATE INDEX idx_terminal_commands_profile_started ON terminal_commands (profile_id, started_at DESC, id DESC)`<br>`CREATE INDEX idx_terminal_commands_terminal_started ON terminal_commands (terminal_id, started_at DESC, id DESC)`<br>`sqlite_autoindex_terminal_commands_1(id)` | 4 | none |
| `terminal_recordings` | `CREATE INDEX idx_terminal_recordings_path ON terminal_recordings (path, id)`<br>`CREATE INDEX idx_terminal_recordings_expires ON terminal_recordings (expires_at, id)`<br>`CREATE INDEX idx_terminal_recordings_profile_started ON terminal_recordings (profile_id, started_at DESC, id DESC)`<br>`CREATE INDEX idx_terminal_recordings_terminal_started ON terminal_recordings (terminal_id, started_at DESC, id DESC)`<br>`sqlite_autoindex_terminal_recordings_1(id)` | 8 | none |

### Workspace static-query scan/sort triage

This is a coverage queue, not a list of defects. Whole-table catalogs, small state tables, JSON argument rowsets, filtered sorts, and recursive CTE scans can be appropriate. Empty-string parameters were used only to compile plans; no timings are inferred from these plans.

| Query | Scan/sort plan detail |
| --- | --- |

## Measured session deletion and FK probes

Fixture: complete baseline global schema, foreign keys enabled, 1,001 sessions, 100,000 historical projected clear traces with matching queue rows, 100,000 heartbeat wake events, and 100,000 heartbeat wake-state rows across 1,000 sessions. Target session has no children. Each sample executes the real parent DELETE inside a savepoint and rolls back outside the measured interval; 21 warm samples, median. `PRAGMA foreign_key_check` passes before measurement.

Baseline median: **32.907 ms**. FK plan: `SCAN session_input_clear_traces`; `SCAN agent_heartbeat_wake_state USING COVERING INDEX sqlite_autoindex_agent_heartbeat_wake_state_1`; `SCAN agent_heartbeat_wake_events`.

| Added index | Median parent DELETE (ms) | Incremental allocated pages (bytes) | FK plan after addition |
| --- | ---: | ---: | --- |
| `CREATE INDEX idx_session_input_clear_traces_session ON session_input_clear_traces(session_id)` | 19.445 | 1,269,760 | `SEARCH session_input_clear_traces USING COVERING INDEX idx_session_input_clear_traces_session (session_id=?)`; `SCAN agent_heartbeat_wake_state USING COVERING INDEX sqlite_autoindex_agent_heartbeat_wake_state_1`; `SCAN agent_heartbeat_wake_events` |
| `CREATE INDEX idx_agent_heartbeat_wake_events_session ON agent_heartbeat_wake_events(session_id)` | 9.127 | 1,269,760 | `SEARCH session_input_clear_traces USING COVERING INDEX idx_session_input_clear_traces_session (session_id=?)`; `SCAN agent_heartbeat_wake_state USING COVERING INDEX sqlite_autoindex_agent_heartbeat_wake_state_1`; `SEARCH agent_heartbeat_wake_events USING COVERING INDEX idx_agent_heartbeat_wake_events_session (session_id=?)` |
| `CREATE INDEX idx_agent_heartbeat_wake_state_session ON agent_heartbeat_wake_state(session_id)` | 0.068 | 1,269,760 | `SEARCH session_input_clear_traces USING COVERING INDEX idx_session_input_clear_traces_session (session_id=?)`; `SEARCH agent_heartbeat_wake_state USING COVERING INDEX idx_agent_heartbeat_wake_state_session (session_id=?)`; `SEARCH agent_heartbeat_wake_events USING COVERING INDEX idx_agent_heartbeat_wake_events_session (session_id=?)` |

Recommendation: append all three indexes in `schema/definitions/20_sessions.sql` (clear traces) and `schema/definitions/10_heartbeat.sql` (wake tables). They preserve uniqueness and row semantics. Each adds one narrow secondary B-tree entry per row; projected traces retain the existing partial pending index because its ordered pending read remains useful. No session table index is added for this fix.

## Measured subagent wake and scoped catalog access

Fixture: complete baseline global schema, FK ON/check clean, 100,000 completed delegated subagents, 1,000 parent sessions/wakes, 10 workspaces. Equal creation timestamps stress the documented id tie-break. Median of 21 warm executions; result materialization included; UPDATE rollback outside timed interval. Scoped page statements isolate the new UNION branch behavior implemented by the session worker.

| Query before indexing | Median ms | Plan |
| --- | ---: | --- |
| wake-read | 12.912 | `SCAN session_subagents; USE TEMP B-TREE FOR ORDER BY` |
| wake-settle | 11.992 | `SCAN session_subagents` |
| workspace-page | 23.907 | `SEARCH session_subagents USING INDEX idx_session_subagents_workspace_status (workspace_id=?); USE TEMP B-TREE FOR ORDER BY` |
| parent-page | 0.414 | `SEARCH session_subagents USING INDEX idx_session_subagents_parent_created (parent_session_id=?); USE TEMP B-TREE FOR LAST TERM OF ORDER BY` |

| Added index | Bytes allocated at 100k rows |
| --- | ---: |
| `CREATE INDEX idx_session_subagents_wake_created ON session_subagents(wake_message_id,created_at,id)` | 2,580,480 |
| `CREATE INDEX idx_session_subagents_workspace_created ON session_subagents(workspace_id,created_at DESC,id DESC)` | 2,490,368 |
| `CREATE INDEX idx_session_subagents_parent_created_id ON session_subagents(parent_session_id,created_at DESC,id DESC)` | 2,580,480 |

| Query after indexing | Median ms | Plan |
| --- | ---: | --- |
| wake-read | 0.615 | `SEARCH session_subagents USING INDEX idx_session_subagents_wake_created (wake_message_id=?)` |
| wake-settle | 0.490 | `SEARCH session_subagents USING INDEX idx_session_subagents_wake_created (wake_message_id=?)` |
| workspace-page | 0.212 | `SEARCH session_subagents USING INDEX idx_session_subagents_workspace_created (workspace_id=?)` |
| parent-page | 0.240 | `SEARCH session_subagents USING INDEX idx_session_subagents_parent_created_id (parent_session_id=?)` |

Recommendation: wake and workspace indexes in `20_sessions.sql`. Parent tie-break improvement is best implemented by extending the existing `idx_session_subagents_parent_created` to `(parent_session_id,created_at,id)` through the owning generator, rather than retaining both prefix-equivalent indexes. Ascending or descending trailing columns can be reverse-scanned for the descending page, provided both directions match. The workload requires workspace or parent scope; a global-created index would add write cost without an authorized API path. Wake updates additionally benefit correlated empty-wake checks and FK parent deletion.

## Measured observability retention and task history

Fixture: complete baseline global schema, FK ON/check clean, 100,000 token-stat rows, 100,000 permission rows, 100,000 historical cleared task blocks, and 100,000 task designation rollups. 1,001 sessions/tasks; target task has zero children. Median of 21 warm executions; savepoint rollback outside timing. Retention cutoffs select no rows to expose recurring empty-sweep cost.

| Query before indexing | Median ms | Plan |
| --- | ---: | --- |
| empty token retention probe | 7.888 | `SCAN CONSTANT ROW; SCALAR SUBQUERY 1; SCAN token_stats` |
| empty token retention DELETE | 18.649 | `SCAN token_stats` |
| empty permission retention probe | 4.522 | `SCAN CONSTANT ROW; SCALAR SUBQUERY 1; SCAN permission_log` |
| empty permission retention DELETE | 4.087 | `SCAN permission_log` |
| historical task blocks page | 3.884 | `SCAN task_blocks; USE TEMP B-TREE FOR ORDER BY` |
| zero-child task DELETE | 14.691 | `SEARCH tasks USING INDEX sqlite_autoindex_tasks_1 (id=?); SCAN task_run_terminal_commands; SEARCH task_runs USING COVERING INDEX idx_task_runs_task_status (task_id=?); SEARCH task_run_reviews USING COVERING INDEX idx_task_run_reviews_task_round_attempt (task_id=?); SEARCH task_profile_capabilities USING COVERING INDEX sqlite_autoindex_task_profile_capabilities_1 (task_id=?); SEARCH task_profile_agents USING COVERING INDEX sqlite_autoindex_task_profile_agents_1 (task_id=?); SEARCH task_execution_profiles USING COVERING INDEX sqlite_autoindex_task_execution_profiles_1 (task_id=?); SEARCH tasks USING COVERING INDEX idx_tasks_parent (parent_task_id=?); SEARCH task_triage_state USING COVERING INDEX idx_task_triage_task (task_id=?); SEARCH task_events USING COVERING INDEX idx_task_events_task_seq (task_id=?); SCAN task_designation_rollups; SEARCH task_dependencies USING COVERING INDEX idx_task_dependencies_depends_on (depends_on_task_id=?); SEARCH task_dependencies USING COVERING INDEX idx_task_dependencies_task (task_id=?); SCAN task_blocks; SEARCH task_block_recurrences USING COVERING INDEX sqlite_autoindex_task_block_recurrences_1 (task_id=?)` |

| Added index | Bytes allocated at 100k rows |
| --- | ---: |
| `CREATE INDEX idx_token_stats_updated ON token_stats(updated_at)` | 1,282,048 |
| `CREATE INDEX idx_perm_timestamp ON permission_log(timestamp)` | 1,282,048 |
| `CREATE INDEX idx_task_blocks_task_created ON task_blocks(task_id,created_at,id)` | 2,580,480 |
| `CREATE INDEX idx_task_designation_rollups_task ON task_designation_rollups(task_id)` | 1,269,760 |

| Query after indexing | Median ms | Plan |
| --- | ---: | --- |
| empty token retention probe | 0.001 | `SCAN CONSTANT ROW; SCALAR SUBQUERY 1; SEARCH token_stats USING COVERING INDEX idx_token_stats_updated (updated_at<?)` |
| empty token retention DELETE | 0.001 | `SEARCH token_stats USING COVERING INDEX idx_token_stats_updated (updated_at<?)` |
| empty permission retention probe | 0.001 | `SCAN CONSTANT ROW; SCALAR SUBQUERY 1; SEARCH permission_log USING COVERING INDEX idx_perm_timestamp (timestamp<?)` |
| empty permission retention DELETE | 0.001 | `SEARCH permission_log USING COVERING INDEX idx_perm_timestamp (timestamp<?)` |
| historical task blocks page | 0.176 | `SEARCH task_blocks USING INDEX idx_task_blocks_task_created (task_id=?)` |
| zero-child task DELETE | 0.035 | `SEARCH tasks USING INDEX sqlite_autoindex_tasks_1 (id=?); SCAN task_run_terminal_commands; SEARCH task_runs USING COVERING INDEX idx_task_runs_task_status (task_id=?); SEARCH task_run_reviews USING COVERING INDEX idx_task_run_reviews_task_round_attempt (task_id=?); SEARCH task_profile_capabilities USING COVERING INDEX sqlite_autoindex_task_profile_capabilities_1 (task_id=?); SEARCH task_profile_agents USING COVERING INDEX sqlite_autoindex_task_profile_agents_1 (task_id=?); SEARCH task_execution_profiles USING COVERING INDEX sqlite_autoindex_task_execution_profiles_1 (task_id=?); SEARCH tasks USING COVERING INDEX idx_tasks_parent (parent_task_id=?); SEARCH task_triage_state USING COVERING INDEX idx_task_triage_task (task_id=?); SEARCH task_events USING COVERING INDEX idx_task_events_task_seq (task_id=?); SEARCH task_designation_rollups USING COVERING INDEX idx_task_designation_rollups_task (task_id=?); SEARCH task_dependencies USING COVERING INDEX idx_task_dependencies_depends_on (depends_on_task_id=?); SEARCH task_dependencies USING COVERING INDEX idx_task_dependencies_task (task_id=?); SEARCH task_blocks USING COVERING INDEX idx_task_blocks_task_created (task_id=?); SEARCH task_block_recurrences USING COVERING INDEX sqlite_autoindex_task_block_recurrences_1 (task_id=?)` |

Recommendation: timestamp indexes enable cheap read-only preflight plus indexed retention deletes. Token stats updates modify its new index on every aggregate refresh (measured fixture overhead below); permission-log timestamp is append-only. Task blocks index covers the observable historical block list, cleared-kind lookup, and FK cascade, unlike the existing partial open index. Rollup index avoids unrelated historical scans on task deletion. `token_usage_daily` already has `(workspace_id,day)` and its primary key begins with `day`, so no additional retention/workspace index is needed. Owning files: `20_sessions.sql`, `21_permissions.sql`, `70_tasks.sql`.

Single token aggregate timestamp update median: 0.002750 ms without the new timestamp index; 0.003750 ms with it. These sub-millisecond microbenchmarks are noisy; the reliable cost is one extra B-tree maintenance operation per changed timestamp, versus removal of a 100k-row scan per sweep.

## Measured newest session Goal projection

Fixture: 100,000 succeeded session-origin loop runs, 1,000 origin identities, one selected workspace, complete baseline schema, foreign keys enabled/check clean, 21 warm samples. The production query retains its `ORDER BY created_at DESC, rowid DESC` tie-break and quarantine subquery.

Baseline 155.273 ms: `SEARCH loop_runs USING INDEX idx_loop_runs_queue_order (workspace_id=?)`; `CORRELATED SCALAR SUBQUERY 1`; `SEARCH control USING INDEX idx_loop_node_controls_quarantined (quarantined=?)`; `USE TEMP B-TREE FOR ORDER BY`.
Indexed 0.003500 ms: `SEARCH loop_runs USING INDEX idx_loop_runs_session_origin_recent (workspace_id=? AND origin_session_id=?)`; `CORRELATED SCALAR SUBQUERY 1`; `SEARCH control USING INDEX idx_loop_node_controls_quarantined (quarantined=?)`.

Recommended DDL in `51_loop_runs.sql`:

```sql
CREATE INDEX idx_loop_runs_session_origin_recent ON loop_runs(workspace_id,origin_session_id,created_at) WHERE origin_kind='session' AND historical=0;
```

Added 2,179,072 allocated bytes for 100,000 indexed rows. `created_at` remains ascending so reversing the B-tree also reverses its implicit rowid suffix, eliminating the sort while preserving the exact existing tie-break. The partial predicate excludes catalog/historical runs and the immutable creation time avoids index rewrites on status/token updates. This directly serves `ReadGoalSessionProjection` and `FindNewestSessionGoal`.

## Reproduction and current recommendation decisions

Run from the worktree root:

```sh
python3 .cache/sqlite-audit/schema_bench.py --ref 84549c6e1fbd691611518df036d3b6165e6ce37c > .cache/sqlite-audit/schema_bench-rerun.json
```

The script creates only in-memory SQLite databases, seeds complete schemas with foreign keys enabled, checks FK integrity, measures 21 executions, and reports plans plus incremental page allocation. `--case session|subagents|tasks|goals` isolates one workload. The first recorded `schema_bench.json` contains the first three fixtures; `schema_bench_goals.json` contains the subsequently added fourth. Timings can vary under the shared machine workload; both runs show the same asymptotic improvements. Python SQLite 3.53.4 is measurement evidence, not a substitute for the owning Go/modernc integration and migration suites.

| Candidate family | Decision | Rationale |
| --- | --- | --- |
| Clear traces + heartbeat session FK indexes (3) | Applied in generated migration 00132 | Real zero-child session deletion scans 300k unrelated rows otherwise. |
| Subagent wake + workspace-created indexes (2) | Applied in generated migration 00132 | Wake settlement/read and scoped catalog demonstrate substantial improvement. |
| Token stats + permission retention timestamp indexes (2) | Applied in generated migration 00132 | Enables cheap empty-sweep preflight plus indexed deletion; coordinated with observability worker. |
| Task block history + designation rollup indexes (2) | Applied in generated migration 00132 | Historical block reads and task deletion demonstrably scale with unrelated history. |
| Newest session Goal partial index (1) | Applied in generated migration 00133 | Eliminates whole-workspace loop scan/sort for a session Goal read. |
| Extend subagent parent-created trailing id | Deferred | Existing parent index already bounds rows; measured 0.414→0.240ms is modest and does not justify duplicate prefix indexes or replacement churn now. |
| Global subagent-created index | Rejected | API requires parent or workspace scope. |
| Token daily retention/workspace index | Rejected | Existing primary key starts with day; `(workspace_id,day)` already exists. |
| Attach expiry partial index | Deferred | Session worker removed opportunistic sweeping from hot listing/aggregate/attach calls; inspect explicit sweep frequency before adding. |
| Loop blob-reference indexes | Rejected in favor of query rewrite | Other-store worker rewrites correlated live-reference scans as a set union; many extra write indexes would be unnecessary. |
| Profile FK indexes and low-cardinality catalog sort indexes | Deferred | No measured hot callsite; a missing complete FK prefix alone is not evidence for an index. |
| Worktree exit/history and loop timetravel/outbox FK indexes | Deferred | Baseline plan has scans, but no scale/frequency evidence yet; compound workspace cleanup may already bound practical rowsets. |
| Session/workspace stream additions | None recommended from schema inventory | Their current queries are principally bounded by existing PK/sequence/timestamp/session indexes; separate workers investigate lifetime and query behavior. |

All index additions preserve schema data and uniqueness. Every additional index has write/storage cost; append-only migration and real Go fresh/reopen/equivalence checks remain the root owner's responsibility. Existing duplicate/prefix indexes were inventoried but not removed speculatively.

## Resolved subagent callsite finding

Runtime inspection confirms `ListUnfinalizedDelegated`, `ListUnfinalizedNative`, and `ListOpenWakes` run once at daemon recovery (`internal/daemon/subagent_reactor.go:39`), not on a polling timer. No recovery-only index is recommended without additional scale evidence. In the baseline, `ListPending` also ran at boot, but `subagentService.successor(parent)` called it on every parent-turn settlement, wake settlement, and queue-full retry (200 ms multiplied by attempt, at most five retries), then filtered other parents in Go (`internal/session/subagent_wake.go:282`). The implemented fix uses a parent-scoped internal query and the existing `idx_session_subagents_parent_delivery`; boot recovery retains the global pending query. This avoids adding an eleventh secondary index for an unnecessarily global hot query. This paragraph records the baseline callsite finding; the owning runtime/store changes and checks are complete.

The audit excludes implementation already supplied by PR #715 (incremental transcript projection/cache/redaction). No replacement of that work is recommended here.

Current-state confirmation: all ten recommended index declarations are present in the root-owned global declarative schema. No session/workspace stream schema, code, historical migration, or generated source was edited by this audit worker. The root generated migrations 00132/00133 and session 00010; the session active-event index came from the separate session-query audit. Final integrated verification is recorded in the main performance report.

## Canonical captured rerun summary

The saved JSON artifacts, rather than exploratory timing rows above, own reproducible report numbers. The Goal fixture was rerun with every origin session assigned to the selected workspace; its baseline latency varied under shared-machine load while its indexed plan remained constant.

| Workload | Baseline median ms | Indexed median ms |
| --- | ---: | ---: |
| session-delete: zero-child DELETE | 26.697417 | 0.052083 |
| subagents: wake read | 11.434917 | 0.436166 |
| subagents: wake settle | 9.744625 | 0.352333 |
| subagents: workspace page | 18.424541 | 0.183750 |
| subagents: parent page | 0.340708 | 0.293208 |
| retention-and-tasks: empty token probe | 4.886583 | 0.001208 |
| retention-and-tasks: empty token DELETE | 6.879916 | 0.001375 |
| retention-and-tasks: empty permission probe | 4.093166 | 0.001083 |
| retention-and-tasks: empty permission DELETE | 4.116000 | 0.001166 |
| retention-and-tasks: historical task blocks | 3.909125 | 0.278625 |
| retention-and-tasks: zero-child task DELETE | 15.241750 | 0.037750 |
| goal-projection: newest session goal | 155.272500 | 0.003500 |
