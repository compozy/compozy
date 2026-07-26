-- +goose Up

CREATE TABLE agent_heartbeat_revisions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			agent_name TEXT NOT NULL,
			source_path TEXT NOT NULL,
			operation TEXT NOT NULL CHECK (operation IN ('write', 'delete', 'rollback')),
			previous_digest TEXT,
			new_digest TEXT,
			new_snapshot_id TEXT REFERENCES agent_heartbeat_snapshots(id) ON DELETE SET NULL,
			body TEXT,
			actor_kind TEXT NOT NULL CHECK (actor_kind IN ('user', 'agent', 'extension', 'system')),
			actor_id TEXT NOT NULL,
			created_at TEXT NOT NULL
		);

CREATE TABLE agent_heartbeat_snapshots (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			agent_name TEXT NOT NULL,
			source_path TEXT NOT NULL,
			schema_version INTEGER NOT NULL DEFAULT 1,
			digest TEXT NOT NULL,
			config_digest TEXT NOT NULL,
			body TEXT NOT NULL,
			frontmatter_json TEXT NOT NULL,
			resolved_json TEXT NOT NULL,
			diagnostics_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE (workspace_id, agent_name, digest)
		);

CREATE TABLE agent_heartbeat_wake_events (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			agent_name TEXT NOT NULL,
			session_id TEXT REFERENCES sessions(id) ON DELETE SET NULL,
			policy_snapshot_id TEXT REFERENCES agent_heartbeat_snapshots(id) ON DELETE SET NULL,
			source TEXT NOT NULL CHECK (source IN ('scheduler', 'manual', 'harness_reentry')),
			result TEXT NOT NULL CHECK (
				result IN ('sent', 'skipped', 'coalesced', 'rate_limited', 'failed')
			),
			reason TEXT NOT NULL CHECK (
				reason IN (
					'wake_sent',
					'heartbeat_disabled',
					'heartbeat_invalid',
					'heartbeat_no_policy',
					'heartbeat_rate_limited',
					'heartbeat_no_eligible_session',
					'cooldown_active',
					'quiet_window',
					'session_not_found',
					'session_unhealthy',
					'session_not_attachable',
					'session_prompt_active',
					'session_prompt_active_race',
					'synthetic_prompt_failed',
					'wake_coalesced'
				)
			),
			synthetic_prompt_id TEXT,
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL
		);

CREATE TABLE agent_heartbeat_wake_state (
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			agent_name TEXT NOT NULL,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			policy_snapshot_id TEXT REFERENCES agent_heartbeat_snapshots(id) ON DELETE SET NULL,
			last_wake_at TEXT,
			next_allowed_at TEXT,
			coalesced_count INTEGER NOT NULL DEFAULT 0 CHECK (coalesced_count >= 0),
			last_result TEXT NOT NULL CHECK (
				last_result IN ('sent', 'skipped', 'coalesced', 'rate_limited', 'failed')
			),
			last_reason TEXT CHECK (
				last_reason IS NULL OR last_reason IN (
					'wake_sent',
					'heartbeat_disabled',
					'heartbeat_invalid',
					'heartbeat_no_policy',
					'heartbeat_rate_limited',
					'heartbeat_no_eligible_session',
					'cooldown_active',
					'quiet_window',
					'session_not_found',
					'session_unhealthy',
					'session_not_attachable',
					'session_prompt_active',
					'session_prompt_active_race',
					'synthetic_prompt_failed',
					'wake_coalesced'
				)
			),
			updated_at TEXT NOT NULL,
			PRIMARY KEY (workspace_id, agent_name, session_id)
		);

CREATE TABLE agent_soul_revisions (
			id               TEXT PRIMARY KEY,
			workspace_id     TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			agent_name       TEXT NOT NULL,
			source_path      TEXT NOT NULL,
			action           TEXT NOT NULL CHECK (action IN ('put', 'delete', 'rollback')),
			previous_digest  TEXT NOT NULL DEFAULT '',
			new_digest       TEXT NOT NULL DEFAULT '',
			body             TEXT NOT NULL DEFAULT '',
			diagnostics_json TEXT NOT NULL DEFAULT '[]',
			actor_kind       TEXT NOT NULL DEFAULT '',
			actor_id         TEXT NOT NULL DEFAULT '',
			origin_kind      TEXT NOT NULL DEFAULT '',
			origin_ref       TEXT NOT NULL DEFAULT '',
			created_at       TEXT NOT NULL
		);

CREATE TABLE agent_soul_snapshots (
			id            TEXT PRIMARY KEY,
			workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			agent_name    TEXT NOT NULL,
			source_path   TEXT NOT NULL,
			digest        TEXT NOT NULL,
			profile_json  TEXT NOT NULL DEFAULT '{}',
			body          TEXT NOT NULL DEFAULT '',
			truncated     INTEGER NOT NULL DEFAULT 0 CHECK (truncated IN (0, 1)),
			created_at    TEXT NOT NULL,
			UNIQUE (workspace_id, agent_name, digest)
		);

CREATE TABLE app_metadata (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);

CREATE TABLE automation_job_catalog_entries (
			job_id                   TEXT PRIMARY KEY REFERENCES automation_jobs(id) ON DELETE CASCADE,
			scope                    TEXT NOT NULL,
			workspace_id             TEXT NOT NULL DEFAULT '',
			source                   TEXT NOT NULL,
			source_rank              INTEGER NOT NULL,
			name                     TEXT NOT NULL,
			loop_name                TEXT NOT NULL DEFAULT '',
			enabled                  BOOLEAN NOT NULL,
			search_name              TEXT NOT NULL,
			search_agent_name        TEXT NOT NULL,
			search_prompt            TEXT NOT NULL,
			search_scope             TEXT NOT NULL,
			search_source            TEXT NOT NULL,
			search_schedule_mode     TEXT NOT NULL,
			search_schedule_expr     TEXT NOT NULL,
			search_schedule_interval TEXT NOT NULL,
			search_schedule_time     TEXT NOT NULL
		);

CREATE TABLE automation_job_overlays (
		job_id            TEXT PRIMARY KEY,
		enabled_override  BOOLEAN NOT NULL,
		updated_at        TEXT NOT NULL
	);

CREATE TABLE automation_jobs (
		id           TEXT PRIMARY KEY,
		scope        TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
		name         TEXT NOT NULL,
		agent_name   TEXT NOT NULL,
		workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		prompt       TEXT NOT NULL,
		schedule     TEXT,
		task         TEXT,
		enabled      BOOLEAN NOT NULL DEFAULT 1,
		retry        TEXT NOT NULL,
		fire_limit   TEXT NOT NULL,
		source       TEXT NOT NULL DEFAULT 'dynamic',
		target_kind  TEXT NOT NULL DEFAULT 'agent' CHECK (target_kind IN ('agent', 'loop')),
		loop_workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		loop_name    TEXT,
		loop_inputs  TEXT,
		loop_input_mapping TEXT,
		created_at   TEXT NOT NULL,
		updated_at   TEXT NOT NULL,
		CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		)
	);

CREATE TABLE automation_runs (
		id         TEXT PRIMARY KEY,
		job_id     TEXT,
		trigger_id TEXT,
		session_id TEXT,
		task_id    TEXT,
		task_run_id TEXT,
		status     TEXT NOT NULL,
		attempt    INTEGER NOT NULL DEFAULT 1,
		started_at TEXT,
		ended_at   TEXT,
		error      TEXT,
		loop_run_id TEXT REFERENCES loop_runs(id) ON DELETE SET NULL
	, fire_id TEXT, scheduled_at TEXT, delivery_error TEXT, delivery_error_at TEXT, metadata_json TEXT NOT NULL DEFAULT '{}');

CREATE TABLE "automation_scheduler_state" (
			job_id                       TEXT PRIMARY KEY,
			next_run_at                  TEXT,
			last_run_at                  TEXT,
			last_scheduled_at            TEXT,
			last_fire_id                 TEXT NOT NULL DEFAULT '',
			schedule_hash                TEXT NOT NULL DEFAULT '',
			catch_up_policy              TEXT NOT NULL DEFAULT 'skip'
				CHECK (catch_up_policy IN ('skip', 'coalesce', 'replay')),
			misfire_grace_seconds        INTEGER NOT NULL DEFAULT 0
				CHECK (misfire_grace_seconds >= 0),
			consecutive_resume_failures  INTEGER NOT NULL DEFAULT 0
				CHECK (consecutive_resume_failures >= 0),
			last_misfire_at              TEXT,
			misfire_count                INTEGER NOT NULL DEFAULT 0
				CHECK (misfire_count >= 0),
			updated_at                   TEXT NOT NULL
		);

CREATE TABLE automation_trigger_catalog_entries (
			trigger_id           TEXT PRIMARY KEY REFERENCES automation_triggers(id) ON DELETE CASCADE,
			scope                TEXT NOT NULL,
			workspace_id         TEXT NOT NULL DEFAULT '',
			event                TEXT NOT NULL,
			source               TEXT NOT NULL,
			source_rank          INTEGER NOT NULL,
			name                 TEXT NOT NULL,
			loop_name            TEXT NOT NULL DEFAULT '',
			enabled              BOOLEAN NOT NULL,
			search_name          TEXT NOT NULL,
			search_agent_name    TEXT NOT NULL,
			search_prompt        TEXT NOT NULL,
			search_scope         TEXT NOT NULL,
			search_source        TEXT NOT NULL,
			search_event         TEXT NOT NULL,
			search_endpoint_slug TEXT NOT NULL,
			search_webhook_id    TEXT NOT NULL
		);

CREATE TABLE automation_trigger_catalog_filter_terms (
			trigger_id TEXT NOT NULL REFERENCES automation_triggers(id) ON DELETE CASCADE,
			value      TEXT NOT NULL,
			PRIMARY KEY (trigger_id, value)
		);

CREATE TABLE automation_trigger_overlays (
		trigger_id        TEXT PRIMARY KEY,
		enabled_override  BOOLEAN NOT NULL,
		updated_at        TEXT NOT NULL
	);

CREATE TABLE automation_triggers (
		id            TEXT PRIMARY KEY,
		scope         TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
		name          TEXT NOT NULL,
		agent_name    TEXT NOT NULL,
		workspace_id  TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		prompt        TEXT NOT NULL,
		event         TEXT NOT NULL,
		filter        TEXT,
		enabled       BOOLEAN NOT NULL DEFAULT 1,
		retry         TEXT NOT NULL,
		fire_limit    TEXT NOT NULL,
		source        TEXT NOT NULL DEFAULT 'dynamic',
		webhook_id    TEXT,
		endpoint_slug TEXT,
		webhook_secret_ref TEXT,
		target_kind   TEXT NOT NULL DEFAULT 'agent' CHECK (target_kind IN ('agent', 'loop')),
		loop_workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		loop_name     TEXT,
		loop_inputs   TEXT,
		loop_input_mapping TEXT,
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL,
		CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		)
	);

CREATE TABLE automation_watch_events (
	seq          INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id       TEXT NOT NULL,
	job_id       TEXT NOT NULL DEFAULT '',
	trigger_id   TEXT NOT NULL DEFAULT '',
	session_id   TEXT NOT NULL DEFAULT '',
	status       TEXT NOT NULL CHECK (status IN ('completed', 'failed')),
	attempt      INTEGER NOT NULL,
	started_at   TEXT,
	ended_at     TEXT,
	error        TEXT NOT NULL DEFAULT '',
	agent_name   TEXT NOT NULL DEFAULT '',
	workspace_id TEXT NOT NULL DEFAULT '',
	retry_json   TEXT NOT NULL DEFAULT ''
);

CREATE TABLE bridge_ingest_dedup (
		idempotency_key    TEXT PRIMARY KEY,
		bridge_instance_id TEXT NOT NULL REFERENCES bridge_instances(id) ON DELETE CASCADE,
		received_at        TEXT NOT NULL,
		expires_at         TEXT NOT NULL
	);

CREATE TABLE bridge_instances (
		id                TEXT PRIMARY KEY,
		scope             TEXT NOT NULL,
		workspace_id      TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		platform          TEXT NOT NULL,
		extension_name    TEXT NOT NULL,
		display_name      TEXT NOT NULL,
		source            TEXT NOT NULL DEFAULT 'dynamic',
		enabled           BOOLEAN NOT NULL DEFAULT 1,
		status            TEXT NOT NULL,
		dm_policy         TEXT NOT NULL DEFAULT 'open',
		routing_policy    TEXT NOT NULL,
		provider_config   TEXT,
		delivery_defaults TEXT,
		notification_suppress BOOLEAN NOT NULL DEFAULT 0,
		degradation_reason TEXT,
		degradation_message TEXT,
		created_at        TEXT NOT NULL,
		updated_at        TEXT NOT NULL
	);

CREATE TABLE bridge_routes (
		routing_key_hash    TEXT PRIMARY KEY,
		scope               TEXT NOT NULL,
		workspace_id        TEXT,
		bridge_instance_id TEXT NOT NULL REFERENCES bridge_instances(id) ON DELETE CASCADE,
		peer_id             TEXT,
		thread_id           TEXT,
		group_id            TEXT,
		session_id          TEXT NOT NULL,
		agent_name          TEXT NOT NULL,
		last_activity_at    TEXT NOT NULL,
		created_at          TEXT NOT NULL,
		updated_at          TEXT NOT NULL
	);

CREATE TABLE bridge_secret_bindings (
		bridge_instance_id TEXT NOT NULL REFERENCES bridge_instances(id) ON DELETE CASCADE,
		binding_name        TEXT NOT NULL,
		secret_ref           TEXT NOT NULL,
		kind                TEXT NOT NULL,
		created_at          TEXT NOT NULL,
		updated_at          TEXT NOT NULL,
		PRIMARY KEY (bridge_instance_id, binding_name)
	);

CREATE TABLE bridge_target_directory (
			bridge_id       TEXT NOT NULL REFERENCES bridge_instances(id) ON DELETE CASCADE,
			canonical_route TEXT NOT NULL,
			display_name    TEXT NOT NULL,
			normalized      TEXT NOT NULL,
			target_type     TEXT NOT NULL CHECK (target_type IN ('channel','user','room','thread','group')),
			qualifier       TEXT NOT NULL DEFAULT '',
			capabilities    TEXT NOT NULL DEFAULT '',
			updated_at      TEXT NOT NULL,
			last_seen_at    TEXT,
			PRIMARY KEY (bridge_id, canonical_route)
		);

CREATE TABLE bridge_target_directory_refresh (
			bridge_id                  TEXT PRIMARY KEY REFERENCES bridge_instances(id) ON DELETE CASCADE,
			last_successful_refresh_at TEXT NOT NULL
		);

CREATE TABLE bridge_task_subscriptions (
			subscription_id    TEXT PRIMARY KEY,
			task_id            TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			bridge_instance_id TEXT NOT NULL REFERENCES bridge_instances(id) ON DELETE CASCADE,
			scope              TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
			workspace_id       TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
			peer_id            TEXT,
			thread_id          TEXT,
			group_id           TEXT,
			delivery_mode      TEXT NOT NULL CHECK (delivery_mode IN ('direct-send', 'reply')),
			created_by_kind    TEXT NOT NULL CHECK (
				created_by_kind IN (
					'human', 'agent_session', 'automation', 'extension', 'network_peer', 'daemon'
				)
			),
			created_by_ref     TEXT NOT NULL,
			created_at         TEXT NOT NULL,
			updated_at         TEXT NOT NULL,
			CHECK (
				(scope = 'global' AND workspace_id IS NULL) OR
				(scope = 'workspace' AND workspace_id IS NOT NULL)
			),
			CHECK (peer_id IS NOT NULL OR group_id IS NOT NULL)
		);

CREATE TABLE config_apply_records (
			id                  TEXT PRIMARY KEY,
			desired_config_hash TEXT NOT NULL,
			active_config_hash  TEXT NOT NULL,
			generation          INTEGER NOT NULL CHECK (generation >= 0),
			actor               TEXT NOT NULL,
			diff_class          TEXT NOT NULL,
			status              TEXT NOT NULL CHECK (status IN ('pending_apply', 'applied', 'blocked', 'failed')),
			diagnostic_json     TEXT NOT NULL DEFAULT '',
			created_at          TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			applied_at          TEXT,
			updated_at          TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

CREATE TABLE event_summaries (
			id                     TEXT PRIMARY KEY,
			session_id             TEXT NOT NULL DEFAULT '',
			workspace_id           TEXT NOT NULL DEFAULT '',
			type                   TEXT NOT NULL,
			agent_name             TEXT NOT NULL DEFAULT '',
			content_json           TEXT NOT NULL DEFAULT '',
			task_id                TEXT NOT NULL DEFAULT '',
			run_id                 TEXT NOT NULL DEFAULT '',
			workflow_id            TEXT NOT NULL DEFAULT '',
			claim_token_hash       TEXT NOT NULL DEFAULT '',
			lease_until            TEXT NOT NULL DEFAULT '',
			coordinator_session_id TEXT NOT NULL DEFAULT '',
			scheduler_reason       TEXT NOT NULL DEFAULT '',
			hook_event             TEXT NOT NULL DEFAULT '',
			hook_name              TEXT NOT NULL DEFAULT '',
			actor_kind             TEXT NOT NULL DEFAULT '',
			actor_id               TEXT NOT NULL DEFAULT '',
			release_reason         TEXT NOT NULL DEFAULT '',
			parent_session_id      TEXT NOT NULL DEFAULT '',
			root_session_id        TEXT NOT NULL DEFAULT '',
			spawn_depth            INTEGER NOT NULL DEFAULT 0,
			summary                TEXT,
			timestamp              TEXT NOT NULL
		, provider TEXT NOT NULL DEFAULT '', outcome TEXT NOT NULL DEFAULT 'info');

CREATE TABLE extensions (
		name          TEXT PRIMARY KEY,
		version       TEXT NOT NULL,
		source        TEXT NOT NULL,
		enabled       BOOLEAN NOT NULL DEFAULT 1,
		manifest_path TEXT NOT NULL,
		installed_at  TEXT NOT NULL,
		capabilities  TEXT NOT NULL DEFAULT '{}',
		actions       TEXT NOT NULL DEFAULT '{}',
		checksum      TEXT NOT NULL,
		registry_slug TEXT,
		registry_name TEXT,
		remote_version TEXT,
		provenance_json TEXT NOT NULL DEFAULT '{}'
	);

CREATE TABLE loop_config (
			workspace_id        TEXT NOT NULL,
			loop_name           TEXT NOT NULL,
			human_gate_enabled  INTEGER NOT NULL DEFAULT 0,
			reattempt_strategy  TEXT,
			enabled_checks_json TEXT NOT NULL DEFAULT '{}',
			iteration_cap       INTEGER,
			budget_tokens       INTEGER,
			budget_wall_sec     INTEGER,
			budget_on_exceeded  TEXT,
			no_progress_window  INTEGER,
			fan_out_width       INTEGER,
			gate_max_revisions  INTEGER,
			model_default_worker TEXT,
			model_default_judge  TEXT,
			PRIMARY KEY (workspace_id, loop_name)
		);

CREATE TABLE loop_definition_snapshots (
			workspace_id        TEXT NOT NULL,
			definition_digest  TEXT NOT NULL,
			definition_version INTEGER NOT NULL DEFAULT 0,
			definition_json    TEXT NOT NULL,
			byte_size          INTEGER NOT NULL CHECK (byte_size >= 0),
			created_at         TEXT NOT NULL,
			last_used_at       TEXT NOT NULL,
			PRIMARY KEY (workspace_id, definition_digest)
		);

CREATE TABLE loop_gate_decisions (
			workspace_id  TEXT NOT NULL,
			loop_run_id   TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
			generation    INTEGER NOT NULL,
			gate_id       TEXT NOT NULL,
			criterion_id  TEXT NOT NULL,
			decision      TEXT NOT NULL,
			actor_kind    TEXT NOT NULL,
			actor_ref     TEXT NOT NULL,
			origin_kind   TEXT NOT NULL,
			origin_ref    TEXT NOT NULL,
			note          TEXT NOT NULL DEFAULT '',
			decided_at    TEXT NOT NULL,
			PRIMARY KEY (loop_run_id, generation, gate_id, criterion_id)
		);

CREATE TABLE loop_generation_outputs (
			loop_run_id       TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
			generation        INTEGER NOT NULL,
			node_id           TEXT NOT NULL,
			item_index        INTEGER NOT NULL DEFAULT 0,
			status            TEXT NOT NULL,
			output_ref        TEXT,
			task_run_id       TEXT,
			child_loop_run_id TEXT, goal_status TEXT, goal_turns_used INTEGER, goal_turn_limit INTEGER,
			PRIMARY KEY (loop_run_id, generation, node_id, item_index)
		);

CREATE TABLE loop_goal_binding_retry_witnesses (
		loop_run_id          TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
		handle               TEXT NOT NULL CHECK (length(trim(handle)) > 0),
		failed_binding_epoch INTEGER NOT NULL CHECK (failed_binding_epoch >= 1),
		request_digest       TEXT NOT NULL CHECK (length(request_digest) = 64),
		created_at           TIMESTAMP NOT NULL,
		PRIMARY KEY (loop_run_id, handle, failed_binding_epoch)
	);

CREATE TABLE loop_goal_checkpoints (
	loop_run_id       TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	generation        INTEGER NOT NULL CHECK (generation >= 1),
	node_id           TEXT NOT NULL CHECK (length(trim(node_id)) > 0),
	item_index        INTEGER NOT NULL DEFAULT 0 CHECK (item_index >= 0),
	control_epoch     INTEGER NOT NULL DEFAULT 1 CHECK (control_epoch >= 1),
	control_actor_kind TEXT,
	control_actor_id  TEXT,
	control_requested_at TIMESTAMP,
	phase             TEXT NOT NULL CHECK (phase IN (
		'idle','preparing','queued','prompting','compacting','judging','persisting','awaiting_control','terminal'
	)),
	goal_status       TEXT NOT NULL CHECK (goal_status IN (
		'active','paused','blocked','usage-limited','budget-limited','complete'
	)),
	turns_used        INTEGER NOT NULL DEFAULT 0 CHECK (turns_used >= 0),
	turn_limit        INTEGER NOT NULL CHECK (turn_limit >= 1),
	broken_streak     INTEGER NOT NULL DEFAULT 0 CHECK (broken_streak >= 0),
	recovery_streak   INTEGER NOT NULL DEFAULT 0 CHECK (recovery_streak >= 0),
	task_run_id       TEXT,
	queue_entry_id    TEXT,
	prompt_id         TEXT,
	prompt_kind       TEXT CHECK (prompt_kind IS NULL OR prompt_kind IN ('work','continuation','compact')),
	prompt_attempt    INTEGER NOT NULL DEFAULT 0 CHECK (prompt_attempt >= 0),
	context_state     TEXT NOT NULL DEFAULT 'unknown' CHECK (context_state IN ('known','unknown','pending')),
	usage_sequence    INTEGER CHECK (usage_sequence IS NULL OR usage_sequence >= 0),
	usage_pending_after_sequence INTEGER CHECK (usage_pending_after_sequence IS NULL OR usage_pending_after_sequence >= 0),
	session_id        TEXT,
	binding_handle    TEXT,
	binding_epoch     INTEGER CHECK (binding_epoch IS NULL OR binding_epoch >= 1),
	context_nudge_ratio REAL NOT NULL CHECK (context_nudge_ratio >= 0.0 AND context_nudge_ratio <= 1.0),
	control_grant_id  INTEGER NOT NULL DEFAULT 0 CHECK (control_grant_id >= 0),
	control_grant_kind TEXT CHECK (
		control_grant_kind IS NULL OR control_grant_kind IN ('turn-extension','budget','reseed','plain-resume')
	),
	control_grant_cause TEXT,
	control_grant_turn INTEGER CHECK (control_grant_turn IS NULL OR control_grant_turn >= 0),
	control_grant_scope TEXT CHECK (
		control_grant_scope IS NULL OR control_grant_scope IN (
			'turn-limit','settle-current','work-and-settle','rotate-binding','reactivate'
		)
	),
	control_grant_consumed INTEGER NOT NULL DEFAULT 1 CHECK (control_grant_consumed IN (0,1)),
	judge_attempt_id  TEXT,
	compaction_cancel_prompt_id TEXT,
	compaction_cancel_cause TEXT CHECK (compaction_cancel_cause IS NULL OR compaction_cancel_cause = 'timeout'),
	compaction_cancel_requested_at TIMESTAMP,
	report_prompt_id  TEXT,
	report_status     TEXT CHECK (report_status IS NULL OR report_status IN ('complete','blocked')),
	report_evidence_ref TEXT,
	report_binding_epoch INTEGER CHECK (report_binding_epoch IS NULL OR report_binding_epoch >= 1),
	report_actor_kind TEXT,
	report_actor_id   TEXT,
	report_recorded_at TIMESTAMP,
	updated_at        TIMESTAMP NOT NULL, control_cause TEXT, compaction_baseline_used INTEGER
				CHECK (compaction_baseline_used IS NULL OR compaction_baseline_used >= 0), compaction_recovery_required INTEGER
				NOT NULL DEFAULT 0 CHECK (compaction_recovery_required IN (0,1)),
	CHECK (
		(context_state = 'known' AND usage_sequence IS NOT NULL AND usage_pending_after_sequence IS NULL)
		OR
		(context_state = 'unknown' AND usage_sequence IS NULL AND usage_pending_after_sequence IS NULL)
		OR
		(context_state = 'pending' AND (
			usage_pending_after_sequence IS NULL OR usage_pending_after_sequence = usage_sequence
		))
	),
	CHECK (
		(control_actor_kind IS NULL AND control_actor_id IS NULL AND control_requested_at IS NULL)
		OR
		(control_actor_kind IS NOT NULL AND control_actor_id IS NOT NULL AND control_requested_at IS NOT NULL)
	),
	CHECK (
		(control_grant_id = 0 AND control_grant_kind IS NULL AND control_grant_cause IS NULL
		 AND control_grant_turn IS NULL AND control_grant_scope IS NULL AND control_grant_consumed = 1)
		OR
		(control_grant_id >= 1 AND control_grant_kind IS NOT NULL AND control_grant_cause IS NOT NULL
		 AND control_grant_scope IS NOT NULL AND control_grant_turn IS NOT NULL)
	),
	CHECK (
		(compaction_cancel_cause IS NULL AND compaction_cancel_prompt_id IS NULL AND compaction_cancel_requested_at IS NULL)
		OR
		(compaction_cancel_cause = 'timeout' AND compaction_cancel_prompt_id IS NOT NULL
		 AND compaction_cancel_requested_at IS NOT NULL)
	),
	CHECK (
		(report_status IS NULL AND report_prompt_id IS NULL AND report_evidence_ref IS NULL
		 AND report_binding_epoch IS NULL AND report_actor_kind IS NULL AND report_actor_id IS NULL
		 AND report_recorded_at IS NULL)
		OR
		(report_status IS NOT NULL AND report_prompt_id IS NOT NULL AND report_binding_epoch IS NOT NULL
		 AND report_actor_kind IS NOT NULL AND report_actor_id IS NOT NULL AND report_recorded_at IS NOT NULL)
	),
	PRIMARY KEY (loop_run_id, generation, node_id, item_index)
);

CREATE TABLE loop_goal_judge_attempts (
	attempt_id        TEXT PRIMARY KEY CHECK (length(trim(attempt_id)) > 0),
	loop_run_id       TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	generation        INTEGER NOT NULL CHECK (generation >= 1),
	node_id           TEXT NOT NULL CHECK (length(trim(node_id)) > 0),
	item_index        INTEGER NOT NULL DEFAULT 0 CHECK (item_index >= 0),
	turn              INTEGER NOT NULL CHECK (turn >= 1),
	judge_digest      TEXT NOT NULL CHECK (length(trim(judge_digest)) > 0),
	status            TEXT NOT NULL CHECK (status IN ('running','completed','ambiguous')),
	outcome           TEXT CHECK (
		outcome IS NULL OR outcome IN ('approved','rejected','blocked','error','timeout','invalid_output')
	),
	blocking_json     TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(blocking_json)),
	evidence_ref      TEXT,
	tokens_used       INTEGER CHECK (tokens_used IS NULL OR tokens_used >= 0),
	usage_base_tokens INTEGER NOT NULL DEFAULT 0 CHECK (usage_base_tokens >= 0),
	started_at        TIMESTAMP NOT NULL,
	completed_at      TIMESTAMP CHECK (completed_at IS NULL OR completed_at >= started_at),
	CHECK (
		(status = 'running' AND outcome IS NULL AND completed_at IS NULL)
		OR
		(status = 'completed' AND outcome IS NOT NULL AND completed_at IS NOT NULL)
		OR
		(status = 'ambiguous' AND outcome IS NULL AND completed_at IS NOT NULL)
	),
	UNIQUE (loop_run_id, generation, node_id, item_index, turn)
);

CREATE TABLE loop_goal_session_cleanup (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			cleanup_id    TEXT NOT NULL UNIQUE CHECK (length(trim(cleanup_id)) > 0),
			workspace_id  TEXT NOT NULL CHECK (length(trim(workspace_id)) > 0),
			loop_run_id   TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
			handle        TEXT NOT NULL CHECK (length(trim(handle)) > 0),
			binding_epoch INTEGER NOT NULL CHECK (binding_epoch >= 1),
			session_id    TEXT NOT NULL CHECK (length(trim(session_id)) > 0),
			cause         TEXT NOT NULL CHECK (cause IN ('terminal','reseed','control-revoked','stop')),
			created_at    TIMESTAMP NOT NULL,
			completed_at  TIMESTAMP CHECK (completed_at IS NULL OR completed_at >= created_at),
			UNIQUE (loop_run_id, handle, binding_epoch)
		);

CREATE TABLE loop_goal_session_outbox (
	id                INTEGER PRIMARY KEY AUTOINCREMENT,
	event_id          TEXT NOT NULL UNIQUE CHECK (length(trim(event_id)) > 0),
	workspace_id      TEXT NOT NULL CHECK (length(trim(workspace_id)) > 0),
	origin_session_id TEXT NOT NULL CHECK (length(trim(origin_session_id)) > 0),
	loop_run_id       TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	bound_session_id  TEXT,
	cause             TEXT NOT NULL CHECK (cause IN ('start','replace','status','clear','reseed')),
	created_at        TIMESTAMP NOT NULL,
	delivered_at      TIMESTAMP CHECK (delivered_at IS NULL OR delivered_at >= created_at)
);

CREATE TABLE loop_goal_turns (
	loop_run_id       TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	seq               INTEGER NOT NULL CHECK (seq >= 1),
	generation        INTEGER NOT NULL CHECK (generation >= 1),
	node_id           TEXT NOT NULL CHECK (length(trim(node_id)) > 0),
	item_index        INTEGER NOT NULL DEFAULT 0 CHECK (item_index >= 0),
	turn              INTEGER NOT NULL CHECK (turn >= 1),
	session_id        TEXT NOT NULL CHECK (length(trim(session_id)) > 0),
	binding_handle    TEXT NOT NULL CHECK (length(trim(binding_handle)) > 0),
	binding_epoch     INTEGER NOT NULL CHECK (binding_epoch >= 1),
	prompt_id         TEXT NOT NULL CHECK (length(trim(prompt_id)) > 0),
	prompt_attempt    INTEGER NOT NULL DEFAULT 0 CHECK (prompt_attempt >= 0),
	usage_base_tokens INTEGER NOT NULL DEFAULT 0 CHECK (usage_base_tokens >= 0),
	result_status     TEXT CHECK (
		result_status IS NULL OR result_status IN ('completed','invalid-result','failed','ambiguous')
	),
	stop_reason       TEXT CHECK (
		stop_reason IS NULL OR stop_reason IN ('end_turn','max_tokens','max_turn_requests','refusal','cancelled')
	),
	reason_code       TEXT,
	verdict_outcome   TEXT CHECK (
		verdict_outcome IS NULL OR verdict_outcome IN ('approved','rejected','blocked','error','timeout','invalid_output')
	),
	blocking_json     TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(blocking_json)),
	evidence_ref      TEXT,
	prompt_ref        TEXT,
	tokens_used       INTEGER CHECK (tokens_used IS NULL OR tokens_used >= 0),
	actor_kind        TEXT NOT NULL CHECK (length(trim(actor_kind)) > 0),
	actor_id          TEXT NOT NULL CHECK (length(trim(actor_id)) > 0),
	started_at        TIMESTAMP NOT NULL,
	ended_at          TIMESTAMP CHECK (ended_at IS NULL OR ended_at >= started_at),
	CHECK (
		(result_status IS NULL AND ended_at IS NULL AND stop_reason IS NULL
		 AND reason_code IS NULL AND verdict_outcome IS NULL)
		OR
		(result_status = 'completed' AND ended_at IS NOT NULL AND stop_reason IS NOT NULL AND reason_code IS NULL)
		OR
		(result_status = 'invalid-result' AND ended_at IS NOT NULL AND stop_reason IS NULL
		 AND reason_code = 'goal_stop_reason_invalid' AND verdict_outcome IS NULL)
		OR
		(result_status = 'failed' AND ended_at IS NOT NULL AND stop_reason IS NULL
		 AND reason_code = 'goal_prompt_request_failed' AND verdict_outcome IS NULL)
		OR
		(result_status = 'ambiguous' AND ended_at IS NOT NULL AND stop_reason IS NULL
		 AND reason_code IN ('goal_recovery_ambiguous','goal_control_revoked_in_flight')
		 AND verdict_outcome IS NULL)
	),
	PRIMARY KEY (loop_run_id, generation, node_id, item_index, turn),
	UNIQUE (loop_run_id, seq),
	UNIQUE (loop_run_id, prompt_id)
);

CREATE TABLE loop_output_blobs (
			output_ref   TEXT PRIMARY KEY,
			payload_json TEXT NOT NULL,
			byte_size    INTEGER NOT NULL CHECK (byte_size >= 0),
			created_at   TEXT NOT NULL,
			last_used_at TEXT NOT NULL
		);

CREATE TABLE loop_run_events (
			id           TEXT PRIMARY KEY,
			loop_run_id  TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			seq          INTEGER NOT NULL,
			kind         TEXT NOT NULL,
			payload_json TEXT NOT NULL,
			at           TIMESTAMP NOT NULL
		);

CREATE TABLE loop_runs (
			id                   TEXT PRIMARY KEY,
			workspace_id         TEXT NOT NULL,
			loop_name            TEXT NOT NULL,
			status               TEXT NOT NULL,
			generation           INTEGER NOT NULL DEFAULT 0,
			reattempt_strategy   TEXT NOT NULL DEFAULT 'failed_only',
			last_progress_at     TIMESTAMP NOT NULL,
			consecutive_failures INTEGER NOT NULL DEFAULT 0,
			budget_tokens        INTEGER NOT NULL DEFAULT 0,
			budget_wall_sec      INTEGER NOT NULL DEFAULT 0,
			budget_on_exceeded   TEXT NOT NULL DEFAULT 'halt',
			tokens_used          INTEGER NOT NULL DEFAULT 0,
			parent_loop_run_id   TEXT,
			pause_requested      INTEGER NOT NULL DEFAULT 0,
			inputs_json          TEXT NOT NULL
		, created_at TEXT NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', iteration_cap INTEGER NOT NULL DEFAULT 0, started_by_kind TEXT NOT NULL DEFAULT '', started_by_ref TEXT NOT NULL DEFAULT '', started_origin_kind TEXT NOT NULL DEFAULT '', started_origin_ref TEXT NOT NULL DEFAULT '', started_at TEXT NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', definition_version INTEGER NOT NULL DEFAULT 0, definition_digest TEXT NOT NULL DEFAULT '', active_gate_id TEXT NOT NULL DEFAULT '', active_human_criteria_json TEXT NOT NULL DEFAULT '[]', budget_approval_seq INTEGER NOT NULL DEFAULT 0, start_metadata_json TEXT NOT NULL DEFAULT '{}', origin_kind TEXT NOT NULL DEFAULT 'catalog', origin_session_id TEXT, goal_cleared_at TIMESTAMP, budget_version INTEGER NOT NULL DEFAULT 0 CHECK (budget_version >= 0), goal_context_nudge_ratio REAL NOT NULL DEFAULT 0.8
				CHECK (goal_context_nudge_ratio >= 0.0 AND goal_context_nudge_ratio <= 1.0), control_actor_kind TEXT, control_actor_id TEXT, control_requested_at TIMESTAMP, origin_creation_profile_ref TEXT
				CHECK (origin_creation_profile_ref IS NULL OR length(trim(origin_creation_profile_ref)) > 0), origin_policy_spec_digest TEXT
				CHECK (origin_policy_spec_digest IS NULL OR length(trim(origin_policy_spec_digest)) > 0), origin_creation_digest TEXT
				CHECK (origin_creation_digest IS NULL OR length(trim(origin_creation_digest)) > 0));

CREATE TABLE loop_session_bindings (
	loop_run_id       TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	handle            TEXT NOT NULL CHECK (length(trim(handle)) > 0),
	binding_epoch     INTEGER NOT NULL CHECK (binding_epoch >= 1),
	binding_attempt_id TEXT NOT NULL UNIQUE CHECK (length(trim(binding_attempt_id)) > 0),
	session_id        TEXT NOT NULL CHECK (length(trim(session_id)) > 0),
	workspace_id      TEXT NOT NULL CHECK (length(trim(workspace_id)) > 0),
	creation_profile_ref TEXT NOT NULL CHECK (length(trim(creation_profile_ref)) > 0),
	policy_spec_digest TEXT NOT NULL CHECK (length(trim(policy_spec_digest)) > 0),
	creation_digest   TEXT NOT NULL CHECK (length(trim(creation_digest)) > 0),
	ownership         TEXT NOT NULL CHECK (ownership IN ('origin-borrowed','run-owned')),
	state             TEXT NOT NULL CHECK (state IN ('creating','active','failed','closed','reseeded')),
	failure_code      TEXT,
	created_at        TIMESTAMP NOT NULL,
	activated_at      TIMESTAMP CHECK (activated_at IS NULL OR activated_at >= created_at),
	failed_at         TIMESTAMP CHECK (failed_at IS NULL OR failed_at >= created_at),
	closed_at         TIMESTAMP CHECK (closed_at IS NULL OR closed_at >= created_at), adopted_generation INTEGER
				NOT NULL DEFAULT 0 CHECK (adopted_generation >= 0), adoption_attempt_id TEXT
			CHECK (adoption_attempt_id IS NULL OR length(trim(adoption_attempt_id)) > 0),
	CHECK (
		(state = 'creating' AND activated_at IS NULL AND failed_at IS NULL AND failure_code IS NULL AND closed_at IS NULL)
		OR
		(state = 'active' AND activated_at IS NOT NULL AND failed_at IS NULL AND failure_code IS NULL AND closed_at IS NULL)
		OR
		(state = 'failed' AND activated_at IS NULL AND failed_at IS NOT NULL
		 AND length(trim(failure_code)) > 0 AND closed_at IS NULL)
		OR
		(state IN ('closed','reseeded') AND activated_at IS NOT NULL AND failed_at IS NULL
		 AND failure_code IS NULL AND closed_at IS NOT NULL)
	),
	CHECK (ownership = 'run-owned' OR (binding_epoch = 1 AND state IN ('active','closed'))),
	PRIMARY KEY (loop_run_id, handle, binding_epoch)
);

CREATE TABLE loop_ui_annotations (
			workspace_id TEXT NOT NULL,
			loop_name    TEXT NOT NULL,
			node_id      TEXT NOT NULL,
			x            REAL NOT NULL,
			y            REAL NOT NULL,
			PRIMARY KEY (workspace_id, loop_name, node_id)
		);

CREATE TABLE mcp_auth_tokens (
			server_name    TEXT PRIMARY KEY,
			issuer         TEXT NOT NULL DEFAULT '',
			client_id      TEXT NOT NULL,
			scopes_json    TEXT NOT NULL DEFAULT '[]',
			access_token_ref   TEXT NOT NULL,
			refresh_token_ref  TEXT NOT NULL DEFAULT '',
			token_type     TEXT NOT NULL DEFAULT 'Bearer',
			expires_at     TEXT,
			obtained_at    TEXT NOT NULL,
			updated_at     TEXT NOT NULL
		);

CREATE TABLE model_catalog_reasoning_efforts (
			source_id   TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			model_id    TEXT NOT NULL,
			effort      TEXT NOT NULL CHECK (trim(effort) <> ''),
			rank        INTEGER NOT NULL CHECK (rank >= 0),
			PRIMARY KEY (source_id, provider_id, model_id, effort),
			FOREIGN KEY (source_id, provider_id, model_id)
				REFERENCES model_catalog_rows(source_id, provider_id, model_id)
				ON DELETE CASCADE
		);

CREATE TABLE model_catalog_rows (
			source_id                TEXT NOT NULL CHECK (trim(source_id) <> ''),
			provider_id              TEXT NOT NULL CHECK (trim(provider_id) <> ''),
			model_id                 TEXT NOT NULL CHECK (trim(model_id) <> ''),
			source_kind              TEXT NOT NULL CHECK (trim(source_kind) <> ''),
			priority                 INTEGER NOT NULL,
			available                INTEGER CHECK (available IN (0, 1) OR available IS NULL),
			stale                    INTEGER NOT NULL DEFAULT 0 CHECK (stale IN (0, 1)),
			refreshed_at             TEXT NOT NULL DEFAULT '',
			expires_at               TEXT NOT NULL DEFAULT '',
			display_name             TEXT NOT NULL DEFAULT '',
			context_window           INTEGER,
			max_input_tokens         INTEGER,
			max_output_tokens        INTEGER,
			supports_tools           INTEGER CHECK (supports_tools IN (0, 1) OR supports_tools IS NULL),
			supports_reasoning       INTEGER CHECK (supports_reasoning IN (0, 1) OR supports_reasoning IS NULL),
			default_reasoning_effort TEXT,
			cost_input_per_million   REAL,
			cost_output_per_million  REAL,
			last_error               TEXT NOT NULL DEFAULT '', deprecated INTEGER NOT NULL DEFAULT 0 CHECK (deprecated IN (0, 1)), hidden INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1)), featured INTEGER NOT NULL DEFAULT 0 CHECK (featured IN (0, 1)), release_date TEXT, explicitly_curated INTEGER NOT NULL DEFAULT 0 CHECK (explicitly_curated IN (0, 1)), deprecated_set INTEGER NOT NULL DEFAULT 0 CHECK (deprecated_set IN (0, 1)), hidden_set INTEGER NOT NULL DEFAULT 0 CHECK (hidden_set IN (0, 1)), featured_set INTEGER NOT NULL DEFAULT 0 CHECK (featured_set IN (0, 1)),
			PRIMARY KEY (source_id, provider_id, model_id),
			FOREIGN KEY (source_id, provider_id)
				REFERENCES model_catalog_sources(source_id, provider_id)
				ON DELETE CASCADE
		);

CREATE TABLE model_catalog_sources (
			source_id       TEXT NOT NULL CHECK (trim(source_id) <> ''),
			provider_id     TEXT NOT NULL CHECK (trim(provider_id) <> ''),
			source_kind     TEXT NOT NULL CHECK (trim(source_kind) <> ''),
			priority        INTEGER NOT NULL,
			refresh_state   TEXT NOT NULL CHECK (trim(refresh_state) <> ''),
			last_refresh_at TEXT NOT NULL DEFAULT '',
			next_refresh_at TEXT NOT NULL DEFAULT '',
			last_success_at TEXT NOT NULL DEFAULT '',
			last_error      TEXT NOT NULL DEFAULT '',
			row_count       INTEGER NOT NULL DEFAULT 0 CHECK (row_count >= 0),
			stale           INTEGER NOT NULL DEFAULT 0 CHECK (stale IN (0, 1)),
			PRIMARY KEY (source_id, provider_id)
		);

CREATE TABLE network_audit_log (
			id         TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			direction  TEXT NOT NULL,
			kind       TEXT NOT NULL,
		channel    TEXT NOT NULL,
		surface    TEXT,
		thread_id  TEXT,
		direct_id  TEXT,
		work_id    TEXT,
		peer_from  TEXT NOT NULL,
		peer_to    TEXT,
		message_id TEXT NOT NULL,
		reason     TEXT,
		size       INTEGER NOT NULL,
		timestamp  TEXT NOT NULL
	);

CREATE TABLE network_channel_kind_counts (
		workspace_id TEXT NOT NULL,
		channel      TEXT NOT NULL,
		kind         TEXT NOT NULL,
		message_count INTEGER NOT NULL CHECK (message_count >= 0),
		PRIMARY KEY (workspace_id, channel, kind)
	);

CREATE TABLE network_channel_participants (
		workspace_id TEXT NOT NULL,
		channel      TEXT NOT NULL,
		peer_id      TEXT NOT NULL,
		PRIMARY KEY (workspace_id, channel, peer_id)
	);

CREATE TABLE network_channel_stats (
		workspace_id                 TEXT NOT NULL,
		channel                      TEXT NOT NULL,
		message_count                INTEGER NOT NULL DEFAULT 0 CHECK (message_count >= 0),
		presence_count               INTEGER NOT NULL DEFAULT 0 CHECK (presence_count >= 0),
		historical_participant_count INTEGER NOT NULL DEFAULT 0 CHECK (historical_participant_count >= 0),
		last_activity_at             TEXT,
		last_presence_at             TEXT,
		last_message_id              TEXT NOT NULL DEFAULT '',
		last_message_preview         TEXT NOT NULL DEFAULT '', last_activity_sequence INTEGER NOT NULL DEFAULT 0, last_presence_sequence INTEGER NOT NULL DEFAULT 0, last_message_sequence INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (workspace_id, channel)
	);

CREATE TABLE network_channels (
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			channel      TEXT NOT NULL,
			purpose      TEXT NOT NULL,
			created_by   TEXT NOT NULL DEFAULT '',
			created_at   TEXT NOT NULL,
			updated_at   TEXT NOT NULL, fanout_policy TEXT NOT NULL DEFAULT 'capability_match' CHECK (
					fanout_policy IN ('capability_match', 'coordinator', 'all_members')
				), coordinator_peer_id TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (workspace_id, channel)
		);

CREATE TABLE network_delivery_guidance_state (
			session_id                   TEXT PRIMARY KEY,
			reply_guidance_delivered     BOOLEAN NOT NULL DEFAULT 0 CHECK (reply_guidance_delivered IN (0, 1)),
			protocol_guidance_delivered  BOOLEAN NOT NULL DEFAULT 0 CHECK (protocol_guidance_delivered IN (0, 1)),
			created_at                   TEXT NOT NULL,
			updated_at                   TEXT NOT NULL
		);

CREATE TABLE network_direct_rooms (
		workspace_id         TEXT NOT NULL,
		channel              TEXT NOT NULL,
		direct_id            TEXT NOT NULL,
		peer_a               TEXT NOT NULL,
		peer_b               TEXT NOT NULL,
		opened_at            TEXT NOT NULL,
		last_activity_at     TEXT NOT NULL,
		message_count        INTEGER NOT NULL DEFAULT 0 CHECK (message_count >= 0),
		open_work_count      INTEGER NOT NULL DEFAULT 0 CHECK (open_work_count >= 0),
		last_message_preview TEXT NOT NULL DEFAULT '', opened_sequence INTEGER NOT NULL DEFAULT 0, last_activity_sequence INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (workspace_id, channel, direct_id),
		UNIQUE (workspace_id, channel, peer_a, peer_b),
		CHECK (peer_a < peer_b)
	);

CREATE TABLE network_subscriptions (
			workspace_id          TEXT NOT NULL,
			channel               TEXT NOT NULL,
			thread_id             TEXT NOT NULL DEFAULT '',
			peer_id               TEXT NOT NULL,
			mode                  TEXT NOT NULL CHECK (mode IN ('mute', 'digest', 'full')),
			keyword_filters_json  TEXT NOT NULL DEFAULT '[]',
			created_at            TEXT NOT NULL,
			updated_at            TEXT NOT NULL,
			PRIMARY KEY (workspace_id, channel, thread_id, peer_id),
			FOREIGN KEY (workspace_id, channel)
				REFERENCES network_channels(workspace_id, channel)
				ON DELETE CASCADE
		);

CREATE TABLE network_task_thread_origins (
			task_id                 TEXT PRIMARY KEY REFERENCES tasks(id) ON DELETE CASCADE,
			workspace_id            TEXT NOT NULL,
			channel                 TEXT NOT NULL,
			thread_id               TEXT NOT NULL,
			origin_message_id       TEXT NOT NULL,
			digest                  TEXT NOT NULL,
			source_message_ids_json TEXT NOT NULL DEFAULT '[]',
			created_at              TEXT NOT NULL,
			updated_at              TEXT NOT NULL,
			FOREIGN KEY (workspace_id, channel, thread_id)
				REFERENCES network_threads(workspace_id, channel, thread_id)
				ON DELETE CASCADE
		);

CREATE TABLE network_thread_participants (
		workspace_id     TEXT NOT NULL,
		channel          TEXT NOT NULL,
		thread_id        TEXT NOT NULL,
		peer_id          TEXT NOT NULL,
		first_message_id TEXT NOT NULL,
		first_seen_at    TEXT NOT NULL,
		last_seen_at     TEXT NOT NULL,
		PRIMARY KEY (workspace_id, channel, thread_id, peer_id),
		FOREIGN KEY (workspace_id, channel, thread_id)
			REFERENCES network_threads(workspace_id, channel, thread_id)
			ON DELETE CASCADE
	);

CREATE TABLE network_thread_peer_token_stats (
			workspace_id             TEXT NOT NULL,
			channel                  TEXT NOT NULL,
			thread_id                TEXT NOT NULL,
			peer_id                  TEXT NOT NULL,
			delivered_count          INTEGER NOT NULL DEFAULT 0 CHECK (delivered_count >= 0),
			prompt_size_bytes        INTEGER NOT NULL DEFAULT 0 CHECK (prompt_size_bytes >= 0),
			estimated_prompt_tokens  INTEGER NOT NULL DEFAULT 0 CHECK (estimated_prompt_tokens >= 0),
			first_delivered_at       TEXT NOT NULL,
			last_delivered_at        TEXT NOT NULL,
			updated_at              TEXT NOT NULL,
			PRIMARY KEY (workspace_id, channel, thread_id, peer_id),
			FOREIGN KEY (workspace_id, channel, thread_id)
				REFERENCES network_threads(workspace_id, channel, thread_id)
				ON DELETE CASCADE
		);

CREATE TABLE network_threads (
		workspace_id         TEXT NOT NULL,
		channel              TEXT NOT NULL,
		thread_id            TEXT NOT NULL,
		root_message_id      TEXT NOT NULL,
		title                TEXT NOT NULL DEFAULT '',
		opened_by_peer_id    TEXT NOT NULL DEFAULT '',
		opened_session_id    TEXT NOT NULL DEFAULT '',
		opened_at            TEXT NOT NULL,
		last_activity_at     TEXT NOT NULL,
		message_count        INTEGER NOT NULL DEFAULT 0 CHECK (message_count >= 0),
		participant_count    INTEGER NOT NULL DEFAULT 0 CHECK (participant_count >= 0),
		open_work_count      INTEGER NOT NULL DEFAULT 0 CHECK (open_work_count >= 0),
		last_message_preview TEXT NOT NULL DEFAULT '', opened_sequence INTEGER NOT NULL DEFAULT 0, last_activity_sequence INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (workspace_id, channel, thread_id)
	);

CREATE TABLE network_timeline_log (
	sequence        INTEGER PRIMARY KEY AUTOINCREMENT,
	message_id      TEXT NOT NULL,
	session_id      TEXT,
	workspace_id    TEXT NOT NULL,
	channel         TEXT NOT NULL,
	surface         TEXT CHECK (surface IN ('thread', 'direct') OR surface IS NULL),
	thread_id       TEXT,
	direct_id       TEXT,
	direction       TEXT NOT NULL,
	peer_from       TEXT NOT NULL,
	peer_to         TEXT,
	kind            TEXT NOT NULL,
	work_id         TEXT,
	reply_to        TEXT,
	trace_id        TEXT,
	causation_id    TEXT,
	intent          TEXT,
	text            TEXT,
	preview_text    TEXT NOT NULL DEFAULT '',
	body_json       TEXT NOT NULL,
	timestamp       TEXT NOT NULL,
	ext_json        TEXT NOT NULL DEFAULT '{}',
	mentions_json   TEXT NOT NULL DEFAULT '[]',
	work_opened     INTEGER NOT NULL DEFAULT 0 CHECK (work_opened IN (0, 1)),
	work_transitioned INTEGER NOT NULL DEFAULT 0 CHECK (work_transitioned IN (0, 1)),
	work_state      TEXT NOT NULL DEFAULT '' CHECK (
		work_state IN ('', 'submitted', 'working', 'needs_input', 'completed', 'failed', 'canceled')
	),
	UNIQUE (workspace_id, message_id),
	CHECK (
		(surface IS NULL AND thread_id IS NULL AND direct_id IS NULL AND work_id IS NULL
			AND kind IN ('greet', 'whois'))
		OR (surface = 'thread' AND thread_id IS NOT NULL AND direct_id IS NULL)
		OR (surface = 'direct' AND direct_id IS NOT NULL AND thread_id IS NULL)
	),
	CHECK (kind IN ('greet', 'whois', 'say', 'capability', 'receipt', 'trace'))
);

CREATE TABLE network_work (
		work_id           TEXT NOT NULL,
		workspace_id      TEXT NOT NULL,
		channel           TEXT NOT NULL,
		surface           TEXT NOT NULL CHECK (surface IN ('thread', 'direct')),
		thread_id         TEXT,
		direct_id         TEXT,
		opened_by_peer_id TEXT NOT NULL,
		opened_session_id TEXT NOT NULL DEFAULT '',
		target_peer_id    TEXT NOT NULL DEFAULT '',
		state             TEXT NOT NULL CHECK (
			state IN ('submitted', 'working', 'needs_input', 'completed', 'failed', 'canceled')
		),
		opened_at         TEXT NOT NULL,
		last_activity_at  TEXT NOT NULL,
		terminal_at       TEXT,
		CHECK (
			(surface = 'thread' AND thread_id IS NOT NULL AND direct_id IS NULL)
			OR (surface = 'direct' AND direct_id IS NOT NULL AND thread_id IS NULL)
		),
		PRIMARY KEY (workspace_id, work_id),
		FOREIGN KEY (workspace_id, channel, thread_id)
			REFERENCES network_threads(workspace_id, channel, thread_id)
			ON DELETE RESTRICT,
		FOREIGN KEY (workspace_id, channel, direct_id)
			REFERENCES network_direct_rooms(workspace_id, channel, direct_id)
			ON DELETE RESTRICT
	);

CREATE TABLE notification_cursors (
			consumer_id       TEXT NOT NULL CHECK (trim(consumer_id) <> ''),
			stream_name       TEXT NOT NULL CHECK (trim(stream_name) <> ''),
			subject_id        TEXT NOT NULL DEFAULT '',
			last_sequence     INTEGER NOT NULL DEFAULT 0 CHECK (last_sequence >= 0),
			last_delivery_id  TEXT NOT NULL DEFAULT '',
			last_delivered_at TEXT,
			last_error        TEXT NOT NULL DEFAULT '',
			updated_at        TEXT NOT NULL,
			PRIMARY KEY (consumer_id, stream_name, subject_id)
		);

CREATE TABLE notification_presets (
			name                     TEXT PRIMARY KEY CHECK (trim(name) <> ''),
			events                   TEXT NOT NULL CHECK (json_valid(events)),
			targets                  TEXT NOT NULL CHECK (json_valid(targets)),
			filter                   TEXT NOT NULL DEFAULT '',
			enabled                  BOOLEAN NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
			built_in                 BOOLEAN NOT NULL DEFAULT 0 CHECK (built_in IN (0, 1)),
			default_version          TEXT NOT NULL DEFAULT '',
			default_hash             TEXT NOT NULL DEFAULT '',
			user_modified            BOOLEAN NOT NULL DEFAULT 0 CHECK (user_modified IN (0, 1)),
			default_update_available BOOLEAN NOT NULL DEFAULT 0 CHECK (default_update_available IN (0, 1)),
			created_at               TEXT NOT NULL,
			updated_at               TEXT NOT NULL
		);

CREATE TABLE permission_log (
		id          TEXT PRIMARY KEY,
		session_id  TEXT NOT NULL REFERENCES sessions(id),
		agent_name  TEXT NOT NULL,
		action      TEXT NOT NULL,
		resource    TEXT NOT NULL,
		decision    TEXT NOT NULL,
		policy_used TEXT NOT NULL,
		timestamp   TEXT NOT NULL
	);

CREATE TABLE resource_records (
		kind        TEXT NOT NULL,
		id          TEXT NOT NULL,
		version     INTEGER NOT NULL,
		scope_kind  TEXT NOT NULL CHECK (scope_kind IN ('global', 'workspace')),
		scope_id    TEXT,
		owner_kind  TEXT NOT NULL,
		owner_id    TEXT NOT NULL,
		source_kind TEXT NOT NULL,
		source_id   TEXT NOT NULL,
		spec_json   TEXT NOT NULL,
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL,
		PRIMARY KEY (kind, id),
		CHECK (
			(scope_kind = 'global' AND scope_id IS NULL) OR
			(scope_kind = 'workspace' AND scope_id IS NOT NULL)
		)
	);

CREATE TABLE resource_source_state (
		source_kind           TEXT NOT NULL,
		source_id             TEXT NOT NULL,
		session_nonce         TEXT NOT NULL,
		last_snapshot_version INTEGER NOT NULL,
		updated_at            TEXT NOT NULL,
		PRIMARY KEY (source_kind, source_id)
	);

CREATE TABLE scheduler_pause (
			id         INTEGER PRIMARY KEY CHECK (id = 1),
			paused     INTEGER NOT NULL DEFAULT 0 CHECK (paused IN (0, 1)),
			paused_by  TEXT NOT NULL DEFAULT '',
			paused_at  TEXT,
			reason     TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

CREATE TABLE session_creation_profiles (
		profile_ref TEXT PRIMARY KEY CHECK (length(trim(profile_ref)) > 0),
		profile_json TEXT NOT NULL CHECK (json_valid(profile_json)),
		created_at TEXT NOT NULL
	);

CREATE TABLE session_health (
			session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			agent_name TEXT NOT NULL,
			state TEXT NOT NULL CHECK (state IN ('idle', 'prompting', 'stopped', 'detached')),
			health TEXT NOT NULL CHECK (health IN ('healthy', 'degraded', 'stale', 'dead', 'unknown')),
			active_prompt BOOLEAN NOT NULL CHECK (active_prompt IN (0, 1)),
			attachable BOOLEAN NOT NULL CHECK (attachable IN (0, 1)),
			eligible_for_wake BOOLEAN NOT NULL CHECK (eligible_for_wake IN (0, 1)),
			ineligibility_reason TEXT,
			last_activity_at TEXT,
			last_presence_at TEXT,
			last_error TEXT,
			updated_at TEXT NOT NULL
		);

CREATE TABLE session_input_queue (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			status TEXT NOT NULL CHECK (status IN ('queued', 'dispatching', 'sent', 'failed', 'canceled')),
			mode TEXT NOT NULL CHECK (mode IN ('queue', 'steer')),
			text TEXT NOT NULL,
			session_generation INTEGER NOT NULL DEFAULT 0,
			task_run_id TEXT NOT NULL DEFAULT '',
			run_generation INTEGER,
			attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
			enqueued_at TEXT NOT NULL,
			dispatch_started_at TEXT,
			sent_at TEXT,
			failed_at TEXT,
			failure_summary TEXT NOT NULL DEFAULT '',
			canceled_at TEXT,
			updated_at TEXT NOT NULL
		, loop_run_id TEXT, owner_kind TEXT, owner_epoch INTEGER, binding_epoch INTEGER, prompt_id TEXT, prompt_kind TEXT, operation_usage_base_tokens INTEGER, prompt_attempt INTEGER NOT NULL DEFAULT 0 CHECK (prompt_attempt >= 0), dispatchable INTEGER NOT NULL DEFAULT 1 CHECK (dispatchable IN (0,1)), activated_at TIMESTAMP, dispatch_token_hash TEXT, fence_kind TEXT, fence_disposition TEXT, fence_reason_code TEXT, fenced_at TIMESTAMP, terminal_event_start_seq INTEGER, terminal_event_end_seq INTEGER, terminal_kind TEXT, terminal_stop_reason TEXT, terminal_disposition TEXT, terminal_reason_code TEXT, terminal_tokens_reported INTEGER NOT NULL DEFAULT 0
				CHECK (terminal_tokens_reported IN (0,1)), terminal_tokens_used INTEGER
				CHECK (terminal_tokens_used IS NULL OR terminal_tokens_used >= 0), terminal_at TIMESTAMP);

CREATE TABLE sessions (
		id             TEXT PRIMARY KEY,
		name           TEXT,
		agent_name     TEXT NOT NULL,
		provider       TEXT NOT NULL DEFAULT '',
		workspace_id   TEXT NOT NULL REFERENCES workspaces(id),
		session_type   TEXT NOT NULL DEFAULT 'user',
		channel          TEXT NOT NULL DEFAULT '',
		state          TEXT NOT NULL,
		acp_session_id TEXT,
		stop_reason    TEXT,
		stop_detail    TEXT,
		subprocess_pid INTEGER NOT NULL DEFAULT 0,
		subprocess_started_at TEXT,
		last_update_at TEXT,
		stall_state    TEXT NOT NULL DEFAULT '',
		stall_reason   TEXT NOT NULL DEFAULT '',
		activity_json  TEXT NOT NULL DEFAULT '',
		attached_to    TEXT NOT NULL DEFAULT '',
		attach_expires_at TEXT,
		transcript_epoch INTEGER NOT NULL DEFAULT 0,
		sandbox_id TEXT NOT NULL DEFAULT '',
		sandbox_backend TEXT NOT NULL DEFAULT 'local',
		sandbox_profile TEXT NOT NULL DEFAULT '',
		sandbox_instance_id TEXT NOT NULL DEFAULT '',
		sandbox_state TEXT NOT NULL DEFAULT '',
		sandbox_provider_state_json TEXT NOT NULL DEFAULT '',
		sandbox_last_sync_at TEXT,
		sandbox_last_sync_error TEXT NOT NULL DEFAULT '',
		created_at     TEXT NOT NULL,
		updated_at     TEXT NOT NULL
	, failure_kind TEXT, failure_summary TEXT NOT NULL DEFAULT '', crash_bundle_path TEXT NOT NULL DEFAULT '', parent_session_id TEXT, root_session_id TEXT, spawn_depth INTEGER NOT NULL DEFAULT 0, spawn_role TEXT, ttl_expires_at TEXT, auto_stop_on_parent BOOLEAN NOT NULL DEFAULT 0, spawn_budget_json TEXT NOT NULL DEFAULT '{}', permission_policy_json TEXT NOT NULL DEFAULT '{}', soul_snapshot_id TEXT
				REFERENCES agent_soul_snapshots(id) ON DELETE SET NULL, soul_digest TEXT NOT NULL DEFAULT '', parent_soul_digest TEXT NOT NULL DEFAULT '', input_generation INTEGER NOT NULL DEFAULT 0, creation_digest TEXT
				CHECK (creation_digest IS NULL OR length(trim(creation_digest)) > 0), policy_spec_digest TEXT
				CHECK (policy_spec_digest IS NULL OR length(trim(policy_spec_digest)) > 0), creation_profile_ref TEXT
				CHECK (creation_profile_ref IS NULL OR length(trim(creation_profile_ref)) > 0));

CREATE TABLE task_block_recurrences (
			task_id    TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			kind       TEXT NOT NULL,
			count      INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (task_id, kind)
		);

CREATE TABLE task_blocks (
			id              TEXT PRIMARY KEY,
			workspace_id    TEXT,
			task_id         TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			kind            TEXT NOT NULL CHECK (kind IN ('needs_input','capability','transient')),
			reason          TEXT NOT NULL CHECK (length(reason) > 0),
			details_json    TEXT,
			created_by_kind TEXT NOT NULL,
			created_by_ref  TEXT NOT NULL,
			created_at      TEXT NOT NULL,
			expires_at      TEXT,
			cleared_at      TEXT,
			cleared_by_kind TEXT,
			cleared_by_ref  TEXT,
			clear_note      TEXT
		);

CREATE TABLE task_dependencies (
		task_id             TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		depends_on_task_id  TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		kind                TEXT NOT NULL CHECK (kind IN ('blocks')),
		created_at          TEXT NOT NULL,
		PRIMARY KEY (task_id, depends_on_task_id, kind),
		CHECK (task_id <> depends_on_task_id)
	);

CREATE TABLE task_designation_rollups (
			designation_group_id  TEXT PRIMARY KEY,
			task_id               TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			summary_json          TEXT NOT NULL,
			created_at            TEXT NOT NULL
		);

CREATE TABLE task_events (
		id          TEXT PRIMARY KEY,
		event_seq   INTEGER NOT NULL,
		task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		run_id      TEXT REFERENCES task_runs(id) ON DELETE SET NULL,
		event_type  TEXT NOT NULL,
		actor_kind  TEXT NOT NULL CHECK (
			actor_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'network_peer', 'daemon'
			)
		),
		actor_id    TEXT NOT NULL,
		origin_kind TEXT NOT NULL CHECK (
			origin_kind IN (
				'cli', 'web', 'uds', 'http', 'automation', 'extension', 'network', 'agent_session', 'daemon'
			)
		),
		origin_ref  TEXT NOT NULL,
		payload_json TEXT,
		timestamp   TEXT NOT NULL
	);

CREATE TABLE task_execution_profiles (
			task_id                  TEXT PRIMARY KEY REFERENCES tasks(id) ON DELETE CASCADE,
			coordinator_mode         TEXT NOT NULL DEFAULT 'inherit' CHECK (
				coordinator_mode IN ('inherit', 'guided')
			),
			coordinator_agent_name   TEXT NOT NULL DEFAULT '',
			coordinator_provider     TEXT NOT NULL DEFAULT '',
			coordinator_model        TEXT NOT NULL DEFAULT '',
			coordinator_guidance     TEXT NOT NULL DEFAULT '',
			worker_mode              TEXT NOT NULL DEFAULT 'inherit' CHECK (
				worker_mode IN ('inherit', 'select')
			),
			worker_agent_name        TEXT NOT NULL DEFAULT '',
			worker_provider          TEXT NOT NULL DEFAULT '',
			worker_model             TEXT NOT NULL DEFAULT '',
			review_agent_name        TEXT NOT NULL DEFAULT '',
			review_provider          TEXT NOT NULL DEFAULT '',
			review_model             TEXT NOT NULL DEFAULT '',
			sandbox_mode             TEXT NOT NULL DEFAULT 'inherit' CHECK (
				sandbox_mode IN ('inherit', 'none', 'ref')
			),
			sandbox_ref              TEXT NOT NULL DEFAULT '',
			created_at               TEXT NOT NULL,
			updated_at               TEXT NOT NULL,
			runtime_mode             TEXT NOT NULL DEFAULT 'default' CHECK (
				runtime_mode IN ('default', 'evidence')
			),
			CHECK (
				(sandbox_mode = 'ref' AND sandbox_ref <> '') OR
				(sandbox_mode <> 'ref' AND sandbox_ref = '')
			)
		);

CREATE TABLE task_profile_agents (
			task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			role        TEXT NOT NULL CHECK (role IN ('worker', 'review', 'participant')),
			preference  TEXT NOT NULL CHECK (preference IN ('required', 'allowed', 'preferred')),
			agent_name  TEXT NOT NULL CHECK (agent_name <> ''),
			PRIMARY KEY (task_id, role, preference, agent_name)
		);

CREATE TABLE task_profile_capabilities (
			task_id       TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			role          TEXT NOT NULL CHECK (role IN ('worker', 'review', 'participant')),
			preference    TEXT NOT NULL CHECK (preference IN ('required', 'preferred')),
			capability_id TEXT NOT NULL CHECK (capability_id <> ''),
			PRIMARY KEY (task_id, role, preference, capability_id)
		);

CREATE TABLE task_profile_channels (
			task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			role        TEXT NOT NULL CHECK (role IN ('review', 'participant')),
			preference  TEXT NOT NULL CHECK (preference IN ('allowed', 'preferred')),
			channel_id  TEXT NOT NULL CHECK (channel_id <> ''),
			PRIMARY KEY (task_id, role, preference, channel_id)
		);

CREATE TABLE task_profile_peers (
			task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			role        TEXT NOT NULL CHECK (role IN ('review', 'participant')),
			preference  TEXT NOT NULL CHECK (preference IN ('allowed', 'preferred')),
			peer_id     TEXT NOT NULL CHECK (peer_id <> ''),
			PRIMARY KEY (task_id, role, preference, peer_id)
		);

CREATE TABLE task_run_idempotency (
		idempotency_key TEXT NOT NULL,
		origin_kind     TEXT NOT NULL CHECK (
			origin_kind IN (
				'cli', 'web', 'uds', 'http', 'automation', 'extension', 'network', 'agent_session', 'daemon'
			)
		),
		origin_ref      TEXT NOT NULL,
		run_id          TEXT NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
		created_at      TEXT NOT NULL,
		PRIMARY KEY (idempotency_key, origin_kind, origin_ref)
	);

CREATE TABLE task_run_preferred_capabilities (
			run_id        TEXT NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
			capability_id TEXT NOT NULL,
			PRIMARY KEY (run_id, capability_id)
		);

CREATE TABLE task_run_required_capabilities (
			run_id        TEXT NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
			capability_id TEXT NOT NULL,
			PRIMARY KEY (run_id, capability_id)
		);

CREATE TABLE task_run_reviews (
		review_id            TEXT PRIMARY KEY,
		task_id              TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		run_id               TEXT NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
		parent_review_id     TEXT REFERENCES task_run_reviews(review_id) ON DELETE SET NULL,
		policy               TEXT NOT NULL CHECK (policy IN ('none', 'on_success', 'on_failure', 'always')),
		review_round         INTEGER NOT NULL CHECK (review_round >= 0),
		attempt              INTEGER NOT NULL CHECK (attempt > 0),
		status               TEXT NOT NULL CHECK (
			status IN ('requested', 'routed', 'in_review', 'recorded', 'circuit_opened', 'canceled')
		),
		outcome              TEXT CHECK (
			outcome IS NULL OR outcome IN (
				'approved', 'rejected', 'blocked', 'error', 'timeout', 'invalid_output'
			)
		),
		confidence           REAL CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1)),
		reason               TEXT NOT NULL DEFAULT '',
		delivery_id          TEXT,
		missing_work_json    TEXT NOT NULL DEFAULT '[]',
		next_round_guidance  TEXT NOT NULL DEFAULT '',
		review_text          TEXT NOT NULL DEFAULT '',
		reviewer_session_id  TEXT,
		reviewer_agent_name  TEXT NOT NULL DEFAULT '',
		reviewer_peer_id     TEXT NOT NULL DEFAULT '',
		reviewer_channel_id  TEXT NOT NULL DEFAULT '',
		reviewed_by_kind     TEXT NOT NULL DEFAULT '',
		reviewed_by_ref      TEXT NOT NULL DEFAULT '',
		requested_at         TEXT NOT NULL,
		routed_at            TEXT,
		started_at           TEXT,
		reviewed_at          TEXT,
		deadline_at          TEXT,
		created_at           TEXT NOT NULL,
		updated_at           TEXT NOT NULL
	);

CREATE TABLE task_run_starvation (
			run_id             TEXT NOT NULL PRIMARY KEY REFERENCES task_runs(id) ON DELETE CASCADE,
			wake_count         INTEGER NOT NULL DEFAULT 0 CHECK (wake_count >= 0),
			first_starved_at   TEXT NOT NULL,
			last_wake_at       TEXT,
			escalation_tier    INTEGER NOT NULL DEFAULT 0 CHECK (escalation_tier >= 0),
			spawn_requested_at TEXT,
			starved_event_at   TEXT,
			updated_at         TEXT NOT NULL
		);

CREATE TABLE "task_runs" (
		id              TEXT PRIMARY KEY,
		task_id         TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		status          TEXT NOT NULL,
		attempt         INTEGER NOT NULL CHECK (attempt > 0),
		previous_run_id TEXT,
		failure_kind    TEXT NOT NULL DEFAULT '' CHECK (
			failure_kind = '' OR failure_kind IN ('operator_forced')
		),
		claimed_by_kind TEXT CHECK (
			claimed_by_kind IS NULL OR claimed_by_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'network_peer', 'daemon'
			)
		),
		claimed_by_ref  TEXT,
		session_id      TEXT,
		origin_kind     TEXT NOT NULL CHECK (
			origin_kind IN (
				'cli', 'web', 'uds', 'http', 'automation', 'extension', 'network', 'agent_session', 'daemon'
			)
		),
		origin_ref      TEXT NOT NULL,
		idempotency_key TEXT,
		network_channel TEXT,
		designation_group_id TEXT NOT NULL DEFAULT '',
		queued_at       TEXT NOT NULL,
		claimed_at      TEXT,
		started_at      TEXT,
		ended_at        TEXT,
		error           TEXT,
		metadata_json   TEXT,
		result_json     TEXT,
		summary         TEXT NOT NULL DEFAULT '',
		claimed_agent_name TEXT NOT NULL DEFAULT '',
		claimed_peer_id TEXT NOT NULL DEFAULT '',
		terminalized_by_session_id TEXT NOT NULL DEFAULT '',
		terminalized_by_agent_name TEXT NOT NULL DEFAULT '',
		terminalized_by_peer_id TEXT NOT NULL DEFAULT '',
		terminalized_by_actor_kind TEXT NOT NULL DEFAULT '',
		terminalized_by_actor_ref TEXT NOT NULL DEFAULT '',
		review_required BOOLEAN NOT NULL DEFAULT 0 CHECK (review_required IN (0, 1)),
		review_request_round INTEGER NOT NULL DEFAULT 0 CHECK (review_request_round >= 0),
		review_policy_snapshot TEXT NOT NULL DEFAULT '' CHECK (
			review_policy_snapshot = '' OR
			review_policy_snapshot IN ('none', 'on_success', 'on_failure', 'always')
		),
		review_request_id TEXT REFERENCES task_run_reviews(review_id),
		parent_run_id TEXT REFERENCES task_runs(id),
		review_id TEXT REFERENCES task_run_reviews(review_id),
		review_round INTEGER NOT NULL DEFAULT 0 CHECK (review_round >= 0),
		continuation_reason TEXT NOT NULL DEFAULT '',
		missing_work_json TEXT NOT NULL DEFAULT '[]',
		next_round_guidance TEXT NOT NULL DEFAULT '',
		claim_token TEXT,
		claim_token_hash TEXT,
		lease_until TEXT,
		heartbeat_at TEXT,
		coordination_channel_id TEXT, run_kind TEXT NOT NULL DEFAULT 'worker', loop_run_id TEXT, tokens_used INTEGER NOT NULL DEFAULT 0,
		CHECK (
			(claimed_by_kind IS NULL AND claimed_by_ref IS NULL) OR
			(claimed_by_kind IS NOT NULL AND claimed_by_ref IS NOT NULL)
		),
		CHECK (status <> 'queued' OR session_id IS NULL)
	);

CREATE TABLE task_triage_state (
		task_id               TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		actor_kind            TEXT NOT NULL CHECK (
			actor_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'network_peer', 'daemon'
			)
		),
		actor_id              TEXT NOT NULL,
		is_read               BOOLEAN NOT NULL DEFAULT 0,
		archived              BOOLEAN NOT NULL DEFAULT 0,
		dismissed             BOOLEAN NOT NULL DEFAULT 0,
		last_seen_activity_at TEXT,
		updated_at            TEXT NOT NULL,
		PRIMARY KEY (task_id, actor_kind, actor_id)
	);

CREATE TABLE "tasks" (
		id              TEXT PRIMARY KEY,
		identifier      TEXT,
		scope           TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
		workspace_id    TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		parent_task_id  TEXT REFERENCES tasks(id),
		network_channel TEXT,
		title           TEXT NOT NULL,
		description     TEXT,
		priority        TEXT NOT NULL DEFAULT 'medium' CHECK (
			priority IN ('low', 'medium', 'high', 'urgent')
		),
		max_attempts    INTEGER NOT NULL DEFAULT 3 CHECK (max_attempts > 0 AND max_attempts <= 10),
		status          TEXT NOT NULL,
		approval_policy TEXT NOT NULL DEFAULT 'none' CHECK (
			approval_policy IN ('none', 'manual')
		),
		approval_state  TEXT NOT NULL DEFAULT 'not_required' CHECK (
			approval_state IN ('not_required', 'pending', 'approved', 'rejected')
		),
		owner_kind      TEXT CHECK (
			owner_kind IS NULL OR owner_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'network_peer', 'pool'
			)
		),
		owner_ref       TEXT,
		created_by_kind TEXT NOT NULL CHECK (
			created_by_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'network_peer', 'daemon'
			)
		),
		created_by_ref  TEXT NOT NULL,
		origin_kind     TEXT NOT NULL CHECK (
			origin_kind IN (
				'cli', 'web', 'uds', 'http', 'automation', 'extension', 'network', 'agent_session', 'daemon'
			)
		),
		origin_ref      TEXT NOT NULL,
		created_at      TEXT NOT NULL,
		updated_at      TEXT NOT NULL,
		closed_at       TEXT,
		metadata_json   TEXT,
		current_run_id  TEXT REFERENCES task_runs(id) ON DELETE SET NULL,
		paused          INTEGER NOT NULL DEFAULT 0 CHECK (paused IN (0, 1)),
		paused_by       TEXT NOT NULL DEFAULT '',
		paused_at       TEXT,
		paused_reason   TEXT NOT NULL DEFAULT '',
		max_runtime_seconds INTEGER NOT NULL DEFAULT 0 CHECK (max_runtime_seconds >= 0),
		spawn_failure_count INTEGER NOT NULL DEFAULT 0 CHECK (spawn_failure_count >= 0),
		last_spawn_error TEXT NOT NULL DEFAULT '',
		review_policy TEXT NOT NULL DEFAULT 'none' CHECK (
			review_policy IN ('none', 'on_success', 'on_failure', 'always')
		),
		review_max_rounds INTEGER NOT NULL DEFAULT 3 CHECK (review_max_rounds >= 0),
		review_round INTEGER NOT NULL DEFAULT 0 CHECK (review_round >= 0),
		last_review_id TEXT,
		last_review_outcome TEXT CHECK (
			last_review_outcome IS NULL OR last_review_outcome IN (
				'approved', 'rejected', 'blocked', 'error', 'timeout', 'invalid_output'
			)
		),
		review_circuit_opened_at TEXT,
		review_circuit_reason TEXT,
		auto_enqueue_on_ready INTEGER NOT NULL DEFAULT 0 CHECK (auto_enqueue_on_ready IN (0, 1)),
		needs_attention_reason  TEXT,
		needs_attention_at      TEXT,
		needs_attention_by_kind TEXT,
		needs_attention_by_ref  TEXT,
		wake_creator            INTEGER NOT NULL DEFAULT 1,
		CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		),
		CHECK (
			(owner_kind IS NULL AND owner_ref IS NULL) OR
			(owner_kind IS NOT NULL AND owner_ref IS NOT NULL)
		),
		CHECK (parent_task_id IS NULL OR parent_task_id <> id),
		CHECK (
			(approval_policy = 'none' AND approval_state = 'not_required') OR
			(approval_policy = 'manual' AND approval_state IN ('pending', 'approved', 'rejected'))
		)
		);

CREATE TABLE token_stats (
		id            TEXT PRIMARY KEY,
		session_id    TEXT NOT NULL REFERENCES sessions(id),
		agent_name    TEXT NOT NULL,
		input_tokens  INTEGER,
		output_tokens INTEGER,
		total_tokens  INTEGER,
		total_cost    REAL,
		cost_currency TEXT,
		turn_count    INTEGER NOT NULL DEFAULT 0,
		updated_at    TEXT NOT NULL
	);

CREATE TABLE tool_processes (
			id               TEXT PRIMARY KEY,
			source           TEXT NOT NULL,
			session_id       TEXT NOT NULL DEFAULT '',
			turn_id          TEXT NOT NULL DEFAULT '',
			tool_call_id     TEXT NOT NULL DEFAULT '',
			terminal_id      TEXT NOT NULL DEFAULT '',
			extension_name   TEXT NOT NULL DEFAULT '',
			hook_name        TEXT NOT NULL DEFAULT '',
			sandbox_id   TEXT NOT NULL DEFAULT '',
			pid              INTEGER NOT NULL DEFAULT 0,
			process_group_id INTEGER NOT NULL DEFAULT 0,
			command          TEXT NOT NULL DEFAULT '',
			args_json        TEXT NOT NULL DEFAULT '[]',
			cwd              TEXT NOT NULL DEFAULT '',
			started_at       TEXT,
			started_by_pid   INTEGER NOT NULL DEFAULT 0,
			state            TEXT NOT NULL,
			exit_code        INTEGER,
			error            TEXT NOT NULL DEFAULT '',
			created_at       TEXT NOT NULL,
			updated_at       TEXT NOT NULL,
			completed_at     TEXT
		);

CREATE TABLE vault_secrets (
			ref             TEXT PRIMARY KEY,
			kind            TEXT NOT NULL DEFAULT '',
			encrypted_value TEXT NOT NULL,
			created_at      TEXT NOT NULL,
			updated_at      TEXT NOT NULL
		);

CREATE TABLE workspaces (
		id            TEXT PRIMARY KEY,
		root_dir      TEXT NOT NULL UNIQUE,
		add_dirs      TEXT NOT NULL DEFAULT '[]',
		name          TEXT NOT NULL UNIQUE,
		default_agent TEXT DEFAULT '',
		sandbox_ref TEXT NOT NULL DEFAULT '',
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL
	);

CREATE INDEX idx_agent_heartbeat_revisions_agent_created
			ON agent_heartbeat_revisions(workspace_id, agent_name, created_at DESC, id DESC);

CREATE INDEX idx_agent_heartbeat_snapshots_agent_created
			ON agent_heartbeat_snapshots(workspace_id, agent_name, created_at DESC, id DESC);

CREATE INDEX idx_agent_heartbeat_wake_events_agent_created
			ON agent_heartbeat_wake_events(workspace_id, agent_name, created_at DESC, id DESC);

CREATE INDEX idx_agent_heartbeat_wake_events_expires
			ON agent_heartbeat_wake_events(expires_at);

CREATE INDEX idx_agent_heartbeat_wake_state_next_allowed
			ON agent_heartbeat_wake_state(next_allowed_at, updated_at DESC);

CREATE INDEX idx_agent_soul_revisions_agent
			ON agent_soul_revisions(workspace_id, agent_name, created_at DESC);

CREATE INDEX idx_agent_soul_snapshots_agent
			ON agent_soul_snapshots(workspace_id, agent_name, created_at DESC);

CREATE INDEX idx_automation_job_catalog_order
			ON automation_job_catalog_entries(source_rank, name, job_id);

CREATE INDEX idx_automation_job_catalog_workspace_order
			ON automation_job_catalog_entries(workspace_id, source_rank, name, job_id);

CREATE INDEX idx_automation_jobs_enabled ON automation_jobs(enabled);

CREATE INDEX idx_automation_jobs_loop_target
			ON automation_jobs(loop_name, loop_workspace_id) WHERE target_kind = 'loop';

CREATE INDEX idx_automation_runs_job ON automation_runs(job_id);

CREATE INDEX idx_automation_runs_loop_run ON automation_runs(loop_run_id);

CREATE INDEX idx_automation_runs_started ON automation_runs(started_at);

CREATE INDEX idx_automation_runs_status ON automation_runs(status);

CREATE INDEX idx_automation_runs_trigger ON automation_runs(trigger_id);

CREATE INDEX idx_automation_scheduler_misfire
			ON automation_scheduler_state(last_misfire_at);

CREATE INDEX idx_automation_scheduler_next_run
			ON automation_scheduler_state(next_run_at);

CREATE INDEX idx_automation_trigger_catalog_order
			ON automation_trigger_catalog_entries(source_rank, name, trigger_id);

CREATE INDEX idx_automation_trigger_catalog_workspace_order
			ON automation_trigger_catalog_entries(workspace_id, source_rank, name, trigger_id);

CREATE INDEX idx_automation_triggers_enabled ON automation_triggers(enabled);

CREATE INDEX idx_automation_triggers_event ON automation_triggers(event);

CREATE INDEX idx_automation_triggers_loop_target
			ON automation_triggers(loop_name, loop_workspace_id) WHERE target_kind = 'loop';

CREATE INDEX idx_automation_watch_events_workspace_status_seq
			ON automation_watch_events(workspace_id, status, seq);

CREATE INDEX idx_bridge_ingest_dedup_expires ON bridge_ingest_dedup(expires_at);

CREATE INDEX idx_bridge_instances_scope ON bridge_instances(scope, workspace_id, id);

CREATE INDEX idx_bridge_routes_instance ON bridge_routes(bridge_instance_id, updated_at DESC);

CREATE INDEX idx_bridge_routes_session ON bridge_routes(session_id);

CREATE INDEX idx_bridge_secret_bindings_instance ON bridge_secret_bindings(bridge_instance_id);

CREATE INDEX idx_bridge_task_subscriptions_bridge
			ON bridge_task_subscriptions(bridge_instance_id, updated_at DESC);

CREATE INDEX idx_bridge_task_subscriptions_scope
			ON bridge_task_subscriptions(scope, workspace_id, updated_at DESC);

CREATE INDEX idx_bridge_task_subscriptions_task
			ON bridge_task_subscriptions(task_id, updated_at DESC);

CREATE INDEX idx_btd_bridge_norm
			ON bridge_target_directory(bridge_id, normalized);

CREATE INDEX idx_btd_bridge_qualifier
			ON bridge_target_directory(bridge_id, qualifier);

CREATE INDEX idx_config_apply_records_active
			ON config_apply_records(active_config_hash, created_at DESC);

CREATE INDEX idx_config_apply_records_actor
			ON config_apply_records(actor, created_at DESC);

CREATE INDEX idx_config_apply_records_desired
			ON config_apply_records(desired_config_hash, created_at DESC);

CREATE INDEX idx_config_apply_records_generation
			ON config_apply_records(generation DESC, created_at DESC);

CREATE INDEX idx_config_apply_records_status
			ON config_apply_records(status, updated_at DESC);

CREATE INDEX idx_loop_gate_decisions_workspace_run
			ON loop_gate_decisions(workspace_id, loop_run_id, generation, gate_id);

CREATE INDEX idx_loop_generation_outputs_output_ref
			ON loop_generation_outputs(output_ref);

CREATE INDEX idx_loop_goal_session_cleanup_pending
			ON loop_goal_session_cleanup(id) WHERE completed_at IS NULL;

CREATE INDEX idx_loop_goal_session_outbox_pending
			ON loop_goal_session_outbox(id) WHERE delivered_at IS NULL;

CREATE INDEX idx_loop_run_events_run_seq
			ON loop_run_events(loop_run_id, seq);

CREATE INDEX idx_loop_runs_catalog
			ON loop_runs(workspace_id, loop_name, created_at DESC, id DESC, status);

CREATE INDEX idx_loop_runs_queue_order
			ON loop_runs(workspace_id, loop_name, status, created_at ASC, id ASC);

CREATE INDEX idx_mcp_auth_tokens_updated_at
			ON mcp_auth_tokens(updated_at);

CREATE INDEX idx_model_catalog_rows_provider_model
			ON model_catalog_rows(provider_id, model_id, priority DESC, refreshed_at DESC, source_id ASC);

CREATE INDEX idx_model_catalog_rows_source_provider
			ON model_catalog_rows(source_id, provider_id);

CREATE INDEX idx_model_catalog_sources_provider
			ON model_catalog_sources(provider_id, refresh_state, stale);

CREATE INDEX idx_net_audit_conversation
			ON network_audit_log(workspace_id, channel, surface, thread_id, direct_id, timestamp);

CREATE INDEX idx_net_audit_ts ON network_audit_log(timestamp);

CREATE INDEX idx_net_audit_work
			ON network_audit_log(workspace_id, work_id, timestamp)
			WHERE work_id IS NOT NULL;

CREATE INDEX idx_net_audit_workspace_session ON network_audit_log(workspace_id, session_id);

CREATE INDEX idx_net_timeline_direct_sequence
		ON network_timeline_log(workspace_id, channel, direct_id, sequence)
		WHERE surface = 'direct';

CREATE INDEX idx_net_timeline_kind_sequence
		ON network_timeline_log(workspace_id, kind, sequence);

CREATE INDEX idx_net_timeline_presence_sequence
		ON network_timeline_log(workspace_id, channel, sequence)
		WHERE surface IS NULL;

CREATE INDEX idx_net_timeline_thread_sequence
		ON network_timeline_log(workspace_id, channel, thread_id, sequence)
		WHERE surface = 'thread';

CREATE INDEX idx_net_timeline_work_sequence
		ON network_timeline_log(workspace_id, work_id, sequence)
		WHERE work_id IS NOT NULL;

CREATE INDEX idx_network_channel_participants_peer
		ON network_channel_participants(workspace_id, peer_id, channel);

CREATE INDEX idx_network_channel_stats_activity
		ON network_channel_stats(workspace_id, last_activity_sequence DESC, channel);

CREATE INDEX idx_network_channels_updated_at ON network_channels(updated_at);

CREATE INDEX idx_network_channels_workspace ON network_channels(workspace_id);

CREATE INDEX idx_network_channels_workspace_updated_at
			ON network_channels(workspace_id, updated_at DESC, channel ASC);

CREATE INDEX idx_network_direct_rooms_activity
		ON network_direct_rooms(workspace_id, channel, last_activity_sequence DESC, direct_id);

CREATE INDEX idx_network_direct_rooms_created
		ON network_direct_rooms(workspace_id, channel, opened_at, direct_id);

CREATE INDEX idx_network_direct_rooms_open_work
		ON network_direct_rooms(workspace_id, channel, open_work_count, last_activity_sequence DESC, direct_id);

CREATE INDEX idx_network_direct_rooms_peer_a
		ON network_direct_rooms(workspace_id, channel, peer_a, last_activity_sequence DESC);

CREATE INDEX idx_network_direct_rooms_peer_b
		ON network_direct_rooms(workspace_id, channel, peer_b, last_activity_sequence DESC);

CREATE INDEX idx_network_subscriptions_peer
			ON network_subscriptions(workspace_id, peer_id, channel, thread_id);

CREATE INDEX idx_network_task_thread_origins_thread
			ON network_task_thread_origins(workspace_id, channel, thread_id, created_at DESC);

CREATE INDEX idx_network_thread_participants_peer
		ON network_thread_participants(workspace_id, peer_id, last_seen_at DESC);

CREATE INDEX idx_network_thread_peer_token_stats_peer
			ON network_thread_peer_token_stats(workspace_id, channel, peer_id, last_delivered_at DESC);

CREATE INDEX idx_network_threads_activity
		ON network_threads(workspace_id, channel, last_activity_sequence DESC, thread_id);

CREATE INDEX idx_network_threads_created
		ON network_threads(workspace_id, channel, opened_sequence, thread_id);

CREATE INDEX idx_network_threads_open_work
		ON network_threads(workspace_id, channel, open_work_count, last_activity_sequence DESC, thread_id);

CREATE INDEX idx_network_threads_title
		ON network_threads(workspace_id, channel, title COLLATE NOCASE, thread_id);

CREATE INDEX idx_network_work_conversation
		ON network_work(workspace_id, channel, surface, thread_id, direct_id, last_activity_at DESC);

CREATE INDEX idx_network_work_state
		ON network_work(workspace_id, state, last_activity_at DESC);

CREATE INDEX idx_notification_presets_builtin
			ON notification_presets(built_in, name);

CREATE INDEX idx_notification_presets_enabled
			ON notification_presets(enabled, name);

CREATE INDEX idx_perm_session ON permission_log(session_id);

CREATE INDEX idx_resource_kind ON resource_records(kind);

CREATE INDEX idx_resource_owner ON resource_records(owner_kind, owner_id, kind);

CREATE INDEX idx_resource_scope ON resource_records(scope_kind, scope_id, kind);

CREATE INDEX idx_resource_source ON resource_records(source_kind, source_id, kind);

CREATE INDEX idx_session_health_wake
			ON session_health(workspace_id, agent_name, eligible_for_wake, active_prompt, attachable);

CREATE INDEX idx_session_health_workspace_agent
			ON session_health(workspace_id, agent_name, health, updated_at DESC);

CREATE INDEX idx_session_input_queue_generation
			ON session_input_queue(session_id, session_generation, status);

CREATE INDEX idx_session_input_queue_goal_owner
			ON session_input_queue(loop_run_id, task_run_id, owner_epoch, status, dispatchable, fence_kind);

CREATE INDEX idx_session_input_queue_pending
			ON session_input_queue(session_id, status, enqueued_at, id);

CREATE INDEX idx_sessions_attach_lock
			ON sessions(attached_to, attach_expires_at);

CREATE INDEX idx_sessions_catalog_activity
			ON sessions(
				workspace_id, state, COALESCE(last_update_at, updated_at) DESC,
				updated_at DESC, created_at DESC, id DESC
			);

CREATE INDEX idx_sessions_catalog_recent
			ON sessions(workspace_id, state, updated_at DESC, created_at DESC, id DESC);

CREATE INDEX idx_sessions_parent ON sessions(parent_session_id);

CREATE INDEX idx_sessions_resumable
			ON sessions(state, failure_kind, last_update_at, updated_at);

CREATE INDEX idx_sessions_root ON sessions(root_session_id);

CREATE INDEX idx_sessions_soul_snapshot
			ON sessions(soul_snapshot_id);

CREATE INDEX idx_sessions_spawn_role ON sessions(spawn_role);

CREATE INDEX idx_sessions_type_depth ON sessions(session_type, spawn_depth);

CREATE INDEX idx_summaries_actor ON event_summaries(actor_kind, actor_id);

CREATE INDEX idx_summaries_hook_event ON event_summaries(hook_event);

CREATE INDEX idx_summaries_outcome_timestamp ON event_summaries(outcome, timestamp DESC);

CREATE INDEX idx_summaries_parent ON event_summaries(parent_session_id);

CREATE INDEX idx_summaries_provider_timestamp ON event_summaries(provider, timestamp DESC);

CREATE INDEX idx_summaries_root ON event_summaries(root_session_id);

CREATE INDEX idx_summaries_run ON event_summaries(run_id);

CREATE INDEX idx_summaries_session ON event_summaries(session_id);

CREATE INDEX idx_summaries_task ON event_summaries(task_id);

CREATE INDEX idx_summaries_timestamp ON event_summaries(timestamp);

CREATE INDEX idx_summaries_type ON event_summaries(type);

CREATE INDEX idx_summaries_workflow ON event_summaries(workflow_id);

CREATE INDEX idx_summaries_workspace ON event_summaries(workspace_id);

CREATE INDEX idx_task_blocks_expiry
			ON task_blocks(expires_at)
			WHERE cleared_at IS NULL AND expires_at IS NOT NULL;

CREATE INDEX idx_task_blocks_open
			ON task_blocks(task_id)
			WHERE cleared_at IS NULL;

CREATE INDEX idx_task_dependencies_depends_on ON task_dependencies(depends_on_task_id, task_id ASC);

CREATE INDEX idx_task_dependencies_task ON task_dependencies(task_id, created_at ASC, depends_on_task_id ASC);

CREATE INDEX idx_task_events_run ON task_events(run_id, timestamp DESC, id DESC);

CREATE INDEX idx_task_events_task ON task_events(task_id, timestamp DESC, id DESC);

CREATE INDEX idx_task_events_task_seq ON task_events(task_id, event_seq ASC);

CREATE INDEX idx_task_events_type ON task_events(event_type, timestamp DESC, id DESC);

CREATE INDEX idx_task_events_type_seq
ON task_events(event_type, event_seq);

CREATE INDEX idx_task_run_idempotency_run ON task_run_idempotency(run_id);

CREATE INDEX idx_task_run_preferred_capabilities_capability
			ON task_run_preferred_capabilities(capability_id, run_id);

CREATE INDEX idx_task_run_required_capabilities_capability
			ON task_run_required_capabilities(capability_id, run_id);

CREATE INDEX idx_task_run_reviews_deadline
		ON task_run_reviews(status, deadline_at);

CREATE INDEX idx_task_run_reviews_reviewer_agent
		ON task_run_reviews(reviewer_agent_name, status);

CREATE INDEX idx_task_run_reviews_reviewer_channel
		ON task_run_reviews(reviewer_channel_id, status);

CREATE INDEX idx_task_run_reviews_reviewer_peer
		ON task_run_reviews(reviewer_peer_id, status);

CREATE INDEX idx_task_run_reviews_reviewer_session
		ON task_run_reviews(reviewer_session_id, status);

CREATE INDEX idx_task_run_reviews_run_status
		ON task_run_reviews(run_id, status);

CREATE INDEX idx_task_run_reviews_task_round_attempt
		ON task_run_reviews(task_id, review_round, attempt);

CREATE INDEX idx_task_run_starvation_tier
			ON task_run_starvation(escalation_tier, run_id);

CREATE INDEX idx_task_runs_active_lease_recovery
			ON task_runs(status, lease_until, heartbeat_at, id);

CREATE INDEX idx_task_runs_channel ON task_runs(network_channel);

CREATE INDEX idx_task_runs_coordination_channel
			ON task_runs(coordination_channel_id, queued_at DESC, id DESC);

CREATE INDEX idx_task_runs_designation_group ON task_runs(task_id, designation_group_id);

CREATE INDEX idx_task_runs_parent_run ON task_runs(parent_run_id);

CREATE INDEX idx_task_runs_pending_claim
			ON task_runs(status, lease_until, queued_at, id);

CREATE INDEX idx_task_runs_previous ON task_runs(previous_run_id);

CREATE INDEX idx_task_runs_review_request
		ON task_runs(review_request_id)
		WHERE review_request_id IS NOT NULL;

CREATE INDEX idx_task_runs_session ON task_runs(session_id);

CREATE INDEX idx_task_runs_session_status
			ON task_runs(session_id, status, lease_until);

CREATE INDEX idx_task_runs_status ON task_runs(status);

CREATE INDEX idx_task_runs_task ON task_runs(task_id, queued_at DESC, id DESC);

CREATE INDEX idx_task_runs_task_review_round
		ON task_runs(task_id, review_round)
		WHERE review_round > 0;

CREATE INDEX idx_task_runs_task_status ON task_runs(task_id, status, queued_at DESC, id DESC);

CREATE INDEX idx_task_triage_actor ON task_triage_state(actor_kind, actor_id, updated_at DESC, task_id);

CREATE INDEX idx_task_triage_task ON task_triage_state(task_id, updated_at DESC);

CREATE INDEX idx_tasks_approval_state ON tasks(approval_state);

CREATE INDEX idx_tasks_channel ON tasks(network_channel);

CREATE INDEX idx_tasks_created_by ON tasks(created_by_kind, created_by_ref);

CREATE INDEX idx_tasks_current_run ON tasks(current_run_id);

CREATE INDEX idx_tasks_owner ON tasks(owner_kind, owner_ref);

CREATE INDEX idx_tasks_parent ON tasks(parent_task_id);

CREATE INDEX idx_tasks_paused ON tasks(paused, updated_at DESC);

CREATE INDEX idx_tasks_priority ON tasks(priority);

CREATE INDEX idx_tasks_review_policy ON tasks(review_policy);

CREATE INDEX idx_tasks_review_round ON tasks(review_round);

CREATE INDEX idx_tasks_scope ON tasks(scope);

CREATE INDEX idx_tasks_status ON tasks(status);

CREATE INDEX idx_tasks_workspace ON tasks(workspace_id);

CREATE INDEX idx_token_stats_session ON token_stats(session_id);

CREATE UNIQUE INDEX idx_token_stats_session_agent ON token_stats(session_id, agent_name);

CREATE INDEX idx_tool_processes_extension
			ON tool_processes(extension_name);

CREATE INDEX idx_tool_processes_hook
			ON tool_processes(hook_name);

CREATE INDEX idx_tool_processes_session_turn
			ON tool_processes(session_id, turn_id);

CREATE INDEX idx_tool_processes_state_updated
			ON tool_processes(state, updated_at);

CREATE INDEX idx_tool_processes_terminal
			ON tool_processes(terminal_id);

CREATE INDEX idx_tool_processes_tool_call
			ON tool_processes(tool_call_id);

CREATE INDEX idx_vault_secrets_kind
			ON vault_secrets(kind);

CREATE INDEX idx_vault_secrets_updated_at
			ON vault_secrets(updated_at);

CREATE INDEX idx_workspaces_name ON workspaces(name);

CREATE INDEX notification_cursors_stream_sequence_idx
			ON notification_cursors(stream_name, last_sequence DESC)
			WHERE last_sequence > 0;

CREATE INDEX task_execution_profiles_task_id_idx
			ON task_execution_profiles(task_id);

CREATE INDEX task_profile_agents_lookup_idx
			ON task_profile_agents(role, preference, agent_name, task_id);

CREATE INDEX task_profile_capabilities_lookup_idx
			ON task_profile_capabilities(role, preference, capability_id, task_id);

CREATE INDEX task_profile_channels_lookup_idx
			ON task_profile_channels(role, preference, channel_id, task_id);

CREATE INDEX task_profile_peers_lookup_idx
			ON task_profile_peers(role, preference, peer_id, task_id);

CREATE UNIQUE INDEX uq_automation_jobs_global_name ON automation_jobs(name) WHERE scope = 'global';

CREATE UNIQUE INDEX uq_automation_jobs_workspace_name ON automation_jobs(workspace_id, name) WHERE scope = 'workspace';

CREATE UNIQUE INDEX uq_automation_runs_fire_id
			ON automation_runs(fire_id) WHERE fire_id IS NOT NULL;

CREATE UNIQUE INDEX uq_automation_triggers_global_name ON automation_triggers(name) WHERE scope = 'global';

CREATE UNIQUE INDEX uq_automation_triggers_webhook_id ON automation_triggers(webhook_id) WHERE webhook_id IS NOT NULL;

CREATE UNIQUE INDEX uq_automation_triggers_workspace_name ON automation_triggers(workspace_id, name) WHERE scope = 'workspace';

CREATE UNIQUE INDEX uq_loop_runs_active_session_goal ON loop_runs(origin_session_id)
			WHERE origin_kind='session'
			  AND status IN ('queued','running','watching','needs-approval','paused');

CREATE UNIQUE INDEX uq_loop_session_bindings_active
			ON loop_session_bindings(loop_run_id, handle) WHERE state='active';

CREATE UNIQUE INDEX uq_session_input_queue_active_steer
			ON session_input_queue(session_id)
			WHERE mode = 'steer' AND status IN ('queued', 'dispatching');

CREATE UNIQUE INDEX uq_session_input_queue_goal_prompt
			ON session_input_queue(loop_run_id, prompt_id)
			WHERE prompt_id IS NOT NULL;

CREATE UNIQUE INDEX uq_task_events_event_seq ON task_events(event_seq);

CREATE UNIQUE INDEX uq_task_run_reviews_delivery
		ON task_run_reviews(review_id, delivery_id)
		WHERE delivery_id IS NOT NULL;

CREATE UNIQUE INDEX uq_task_run_reviews_reviewer_session_active
		ON task_run_reviews(reviewer_session_id)
		WHERE reviewer_session_id IS NOT NULL AND status IN ('routed', 'in_review');

CREATE UNIQUE INDEX uq_task_run_reviews_run_round_attempt
		ON task_run_reviews(run_id, review_round, attempt);

CREATE UNIQUE INDEX uq_task_runs_active_loop_coordinator
			ON task_runs(loop_run_id)
			WHERE run_kind = 'coordinator' AND status IN ('queued', 'claimed', 'starting', 'running');

CREATE UNIQUE INDEX uq_task_runs_review_id
		ON task_runs(review_id)
		WHERE review_id IS NOT NULL;

-- +goose StatementBegin
CREATE TRIGGER automation_watch_events_after_insert
AFTER INSERT ON automation_runs
WHEN NEW.status IN ('completed', 'failed')
BEGIN
	INSERT INTO automation_watch_events (
		run_id, job_id, trigger_id, session_id, status, attempt,
		started_at, ended_at, error, agent_name, workspace_id, retry_json
	) VALUES (
		NEW.id,
		COALESCE(NEW.job_id, ''),
		COALESCE(NEW.trigger_id, ''),
		COALESCE(NEW.session_id, ''),
		NEW.status,
		NEW.attempt,
		NEW.started_at,
		NEW.ended_at,
		COALESCE(NEW.error, ''),
		COALESCE(
			(SELECT agent_name FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT agent_name FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		),
		COALESCE(
			NULLIF((SELECT workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			NULLIF((SELECT workspace_id FROM loop_runs WHERE id = NEW.loop_run_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			''
		),
		COALESCE(
			(SELECT retry FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT retry FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		)
	);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER automation_watch_events_after_terminal_update
AFTER UPDATE OF status ON automation_runs
WHEN NEW.status IN ('completed', 'failed') AND OLD.status NOT IN ('completed', 'failed')
BEGIN
	INSERT INTO automation_watch_events (
		run_id, job_id, trigger_id, session_id, status, attempt,
		started_at, ended_at, error, agent_name, workspace_id, retry_json
	) VALUES (
		NEW.id,
		COALESCE(NEW.job_id, ''),
		COALESCE(NEW.trigger_id, ''),
		COALESCE(NEW.session_id, ''),
		NEW.status,
		NEW.attempt,
		NEW.started_at,
		NEW.ended_at,
		COALESCE(NEW.error, ''),
		COALESCE(
			(SELECT agent_name FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT agent_name FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		),
		COALESCE(
			NULLIF((SELECT workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			NULLIF((SELECT workspace_id FROM loop_runs WHERE id = NEW.loop_run_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			''
		),
		COALESCE(
			(SELECT retry FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT retry FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		)
	);
END;
-- +goose StatementEnd

INSERT OR IGNORE INTO scheduler_pause (id, paused, paused_by, reason) VALUES (1, 0, '', '');
