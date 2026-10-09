CREATE TABLE session_input_clear_traces (
    entry_id TEXT PRIMARY KEY REFERENCES session_input_queue(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    turn_id TEXT NOT NULL,
    actor_kind TEXT NOT NULL CHECK (length(trim(actor_kind)) > 0),
    actor_id TEXT NOT NULL CHECK (length(trim(actor_id)) > 0),
    queue_generation INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    projected_at TEXT
);

CREATE INDEX idx_session_input_clear_traces_pending
    ON session_input_clear_traces(session_id, created_at, entry_id) WHERE projected_at IS NULL;

CREATE TABLE session_creation_profiles (
		profile_ref TEXT PRIMARY KEY CHECK (length(trim(profile_ref)) > 0),
		profile_json TEXT NOT NULL CHECK (json_valid(profile_json)),
		created_at TEXT NOT NULL
	);

CREATE TABLE session_health (
			session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
			workspace_id TEXT NOT NULL,
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

	CREATE TABLE session_prompt_admissions (
		id TEXT NOT NULL PRIMARY KEY CHECK (length(trim(id)) > 0),
		workspace_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		message_id TEXT NOT NULL CHECK (length(trim(message_id)) > 0),
		idempotency_key TEXT NOT NULL CHECK (length(trim(idempotency_key)) > 0),
		operation TEXT NOT NULL CHECK (operation IN ('prompt', 'steer')),
		fingerprint_version TEXT NOT NULL CHECK (length(trim(fingerprint_version)) > 0),
		request_fingerprint TEXT NOT NULL CHECK (length(trim(request_fingerprint)) > 0),
		state TEXT NOT NULL CHECK (state IN ('reserved', 'dispatch_committed', 'completed', 'indeterminate')),
		mode TEXT NOT NULL DEFAULT '',
		authored_text TEXT NOT NULL DEFAULT '',
		skill_invocations_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(skill_invocations_json)),
		attachments_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(attachments_json)),
		runtime_provider TEXT NOT NULL DEFAULT '',
		runtime_model TEXT NOT NULL DEFAULT '',
		runtime_reasoning_effort TEXT NOT NULL DEFAULT '',
		runtime_speed TEXT NOT NULL DEFAULT '',
		runtime_acp_options_json TEXT NOT NULL DEFAULT '[]'
			CHECK (json_valid(runtime_acp_options_json)),
		turn_id TEXT NOT NULL CHECK (length(trim(turn_id)) > 0),
		event_id TEXT NOT NULL CHECK (length(trim(event_id)) > 0),
		result_json TEXT CHECK (result_json IS NULL OR json_valid(result_json)),
		indeterminate_reason TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		dispatch_committed_at TEXT,
		completed_at TEXT,
		updated_at TEXT NOT NULL,
		FOREIGN KEY (workspace_id, session_id) REFERENCES sessions(workspace_id, id) ON DELETE CASCADE,
		UNIQUE (workspace_id, session_id, idempotency_key),
		UNIQUE (workspace_id, session_id, message_id)
	);

	CREATE TABLE session_input_queue (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			prompt_admission_id TEXT REFERENCES session_prompt_admissions(id) ON DELETE SET NULL,
			message_id TEXT NOT NULL DEFAULT '',
			idempotency_key TEXT NOT NULL DEFAULT '',
			turn_id TEXT NOT NULL DEFAULT '',
			target_turn_id TEXT NOT NULL DEFAULT '',
			event_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL CHECK (status IN ('queued', 'dispatching', 'sent', 'failed', 'canceled')),
			mode TEXT NOT NULL CHECK (mode IN ('queue', 'steer', 'interrupt')),
			delivery TEXT NOT NULL DEFAULT 'after_turn'
				CHECK (delivery IN ('after_turn', 'interrupt_then_prompt')),
			steer_delivery TEXT CHECK (steer_delivery IN ('injected', 'pending_injection', 'interrupt_fallback')),
			text TEXT NOT NULL,
			synthetic_prompt_json TEXT CHECK (synthetic_prompt_json IS NULL OR json_valid(synthetic_prompt_json)),
			skill_invocations_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(skill_invocations_json)),
			attachments_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(attachments_json)),
			runtime_provider TEXT NOT NULL DEFAULT '',
			runtime_model TEXT NOT NULL DEFAULT '',
			runtime_reasoning_effort TEXT NOT NULL DEFAULT '',
			runtime_speed TEXT NOT NULL DEFAULT '',
			runtime_acp_options_json TEXT NOT NULL DEFAULT '[]'
			CHECK (json_valid(runtime_acp_options_json)),
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
				CHECK (terminal_tokens_used IS NULL OR terminal_tokens_used >= 0), terminal_at TIMESTAMP, priority INTEGER NOT NULL DEFAULT 0);

CREATE TABLE sessions (
		id             TEXT PRIMARY KEY,
		profile_id     TEXT NOT NULL REFERENCES profiles(id),
		name           TEXT,
		agent_name     TEXT NOT NULL,
		provider       TEXT NOT NULL DEFAULT '',
		model          TEXT NOT NULL DEFAULT '',
		reasoning_effort TEXT NOT NULL DEFAULT '',
		speed          TEXT NOT NULL DEFAULT '',
		acp_options_json TEXT NOT NULL DEFAULT '[]'
			CHECK (json_valid(acp_options_json)),
		speed_resolution_json TEXT NOT NULL DEFAULT '',
		runtime_status TEXT NOT NULL DEFAULT 'unbound',
		runtime_transition TEXT NOT NULL DEFAULT '',
		runtime_failure TEXT NOT NULL DEFAULT '',
		runtime_generation INTEGER NOT NULL DEFAULT 0 CHECK (runtime_generation >= 0),
		runtime_recovery_json TEXT NOT NULL DEFAULT ''
			CHECK (runtime_recovery_json = '' OR json_valid(runtime_recovery_json)),
		selected_provider TEXT NOT NULL DEFAULT '',
		selected_model TEXT NOT NULL DEFAULT '',
		selected_reasoning_effort TEXT NOT NULL DEFAULT '',
		selected_speed TEXT NOT NULL DEFAULT '',
		selected_acp_options_json TEXT NOT NULL DEFAULT '[]'
			CHECK (json_valid(selected_acp_options_json)),
		runtime_selection_revision INTEGER NOT NULL DEFAULT 0,
		workspace_id   TEXT NOT NULL,
		scope          TEXT NOT NULL DEFAULT 'workspace' CHECK (scope IN ('global', 'workspace')),
		worktree_id    TEXT,
		session_type   TEXT NOT NULL DEFAULT 'user',
		state          TEXT NOT NULL,
		archived_at    TEXT,
		acp_session_id TEXT,
		stop_reason    TEXT,
		stop_escalated BOOLEAN NOT NULL DEFAULT FALSE CHECK (stop_escalated IN (0, 1)),
		stop_verification_failed BOOLEAN NOT NULL DEFAULT FALSE CHECK (stop_verification_failed IN (0, 1)),
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
		pending_permission_count INTEGER NOT NULL DEFAULT 0 CHECK (pending_permission_count >= 0),
		pending_clarify_count INTEGER NOT NULL DEFAULT 0 CHECK (pending_clarify_count >= 0),
		attention_revision INTEGER NOT NULL DEFAULT 0 CHECK (attention_revision >= 0),
		last_settled_revision INTEGER NOT NULL DEFAULT 0 CHECK (last_settled_revision >= 0),
		last_seen_revision INTEGER NOT NULL DEFAULT 0 CHECK (last_seen_revision >= 0),
		last_seen_at TEXT,
		attention_changed_at TEXT,
		created_at     TEXT NOT NULL,
		updated_at     TEXT NOT NULL
	, failure_kind TEXT, failure_summary TEXT NOT NULL DEFAULT '', crash_bundle_path TEXT NOT NULL DEFAULT '', parent_session_id TEXT, root_session_id TEXT, spawn_depth INTEGER NOT NULL DEFAULT 0, spawn_role TEXT, ttl_expires_at TEXT, auto_stop_on_parent BOOLEAN NOT NULL DEFAULT 0, notify_creator BOOLEAN NOT NULL DEFAULT 1, spawn_budget_json TEXT NOT NULL DEFAULT '{}', permission_policy_json TEXT NOT NULL DEFAULT '{}', soul_snapshot_id TEXT
				REFERENCES agent_soul_snapshots(id) ON DELETE SET NULL, soul_digest TEXT NOT NULL DEFAULT '', parent_soul_digest TEXT NOT NULL DEFAULT '', input_generation INTEGER NOT NULL DEFAULT 0, creation_digest TEXT
				CHECK (creation_digest IS NULL OR length(trim(creation_digest)) > 0), policy_spec_digest TEXT
				CHECK (policy_spec_digest IS NULL OR length(trim(policy_spec_digest)) > 0), creation_profile_ref TEXT
				CHECK (creation_profile_ref IS NULL OR length(trim(creation_profile_ref)) > 0), lineage_kind TEXT NOT NULL DEFAULT ''
				CHECK (lineage_kind IN ('', 'provenance', 'spawn', 'continue', 'fork', 'recovery')), origin_message_id TEXT, origin_agent_name TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (workspace_id, worktree_id)
			REFERENCES worktrees(workspace_id, id),
		CHECK ((scope = 'workspace') = (workspace_id <> '')),
		UNIQUE (workspace_id, id));

CREATE TABLE session_derivations (
		workspace_id TEXT NOT NULL,
		idempotency_key TEXT NOT NULL,
		profile_id TEXT NOT NULL,
		request_fingerprint TEXT NOT NULL,
		source_session_id TEXT NOT NULL,
		child_session_id TEXT NOT NULL,
		kind TEXT NOT NULL CHECK (kind IN ('continue', 'fork')),
		outcome_json TEXT NOT NULL CHECK (json_valid(outcome_json)),
		created_at TEXT NOT NULL,
		child_deleted_at TEXT,
		PRIMARY KEY (workspace_id, idempotency_key)
	);

CREATE INDEX idx_session_derivations_source ON session_derivations(source_session_id);

CREATE INDEX idx_session_derivations_child ON session_derivations(child_session_id);

CREATE TABLE session_pending_interactions (
		interaction_id TEXT PRIMARY KEY CHECK (length(trim(interaction_id)) > 0),
		session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		kind TEXT NOT NULL CHECK (kind IN ('permission', 'clarify')),
		provider_request_id TEXT NOT NULL CHECK (length(trim(provider_request_id)) > 0),
		turn_id TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL DEFAULT '' CHECK (length(CAST(title AS BLOB)) <= 200),
		payload_json TEXT NOT NULL DEFAULT '{}'
			CHECK (json_valid(payload_json) AND length(CAST(payload_json AS BLOB)) <= 4096),
		status TEXT NOT NULL CHECK (status IN ('pending', 'orphaned', 'resolved', 'timed_out', 'canceled')),
		created_at TEXT NOT NULL,
		resolved_at TEXT,
		resolution TEXT NOT NULL DEFAULT '' CHECK (length(CAST(resolution AS BLOB)) <= 240),
		resolved_by TEXT NOT NULL DEFAULT ''
	);

CREATE TABLE token_stats (
		cache_read_tokens INTEGER,
		cache_write_tokens INTEGER,
		id            TEXT PRIMARY KEY,
		session_id    TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		agent_name    TEXT NOT NULL,
		input_tokens  INTEGER,
		output_tokens INTEGER,
		total_tokens  INTEGER,
		total_cost    REAL,
		cost_currency TEXT,
		cost_status   TEXT NOT NULL DEFAULT 'unknown'
			CHECK (cost_status IN ('actual', 'estimated', 'included', 'unknown')),
		cost_source   TEXT NOT NULL DEFAULT 'none'
			CHECK (cost_source IN ('agent_reported', 'catalog_config', 'models_dev', 'builtin', 'none')),
		turn_count    INTEGER NOT NULL DEFAULT 0,
		updated_at    TEXT NOT NULL
	);

CREATE TABLE token_usage_daily (
		day           TEXT NOT NULL CHECK (length(day) = 10),
		profile_id    TEXT NOT NULL REFERENCES profiles(id),
		workspace_id  TEXT NOT NULL DEFAULT '',
		agent_name    TEXT NOT NULL DEFAULT '',
		input_tokens  INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
		output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
		total_tokens  INTEGER NOT NULL DEFAULT 0 CHECK (total_tokens >= 0),
		total_cost    REAL,
		cost_currency TEXT,
		cost_status   TEXT NOT NULL DEFAULT 'unknown'
			CHECK (cost_status IN ('actual', 'estimated', 'included', 'unknown')),
		cost_source   TEXT NOT NULL DEFAULT 'none'
			CHECK (cost_source IN ('agent_reported', 'catalog_config', 'models_dev', 'builtin', 'none')),
		turn_count    INTEGER NOT NULL DEFAULT 0 CHECK (turn_count >= 0),
		updated_at    TEXT NOT NULL,
		PRIMARY KEY (day, profile_id, workspace_id, agent_name)
	);

CREATE INDEX idx_token_usage_daily_profile_day
	ON token_usage_daily (profile_id, day);

CREATE INDEX idx_session_health_wake
			ON session_health(workspace_id, agent_name, eligible_for_wake, active_prompt, attachable);

CREATE INDEX idx_session_health_workspace_agent
			ON session_health(workspace_id, agent_name, health, updated_at DESC);

CREATE INDEX idx_session_input_queue_generation
			ON session_input_queue(session_id, session_generation, status);

CREATE INDEX idx_session_input_queue_goal_owner
			ON session_input_queue(loop_run_id, task_run_id, owner_epoch, status, dispatchable, fence_kind);

	CREATE INDEX idx_session_input_queue_pending
			ON session_input_queue(session_id, status, delivery DESC, priority DESC, enqueued_at ASC, id ASC);

	CREATE UNIQUE INDEX uq_session_input_queue_prompt_admission
			ON session_input_queue(prompt_admission_id)
			WHERE prompt_admission_id IS NOT NULL;

	CREATE INDEX idx_session_prompt_admissions_state
			ON session_prompt_admissions(workspace_id, session_id, state, updated_at);

CREATE INDEX idx_sessions_attach_lock
			ON sessions(attached_to, attach_expires_at);

CREATE INDEX idx_sessions_global_catalog_recent
 ON sessions(archived_at, updated_at DESC, created_at DESC, id DESC);

CREATE INDEX idx_sessions_global_catalog_activity
 ON sessions(archived_at, COALESCE(last_update_at, updated_at) DESC, updated_at DESC, created_at DESC, id DESC);

CREATE INDEX idx_sessions_global_catalog_attention
 ON sessions(archived_at, (CASE
 WHEN (stop_verification_failed = 1 AND state <> 'stopped')
 OR pending_permission_count > 0 OR pending_clarify_count > 0
 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure')
 OR (state = 'stopped' AND trim(COALESCE(failure_kind, '')) <> '' AND trim(COALESCE(failure_kind, '')) <> 'cancellation') THEN 0
 WHEN state = 'active' AND trim(COALESCE(stall_state, '')) <> 'stalled'
 AND trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) = ''
 AND last_settled_revision > last_seen_revision THEN 1
 ELSE 2 END), COALESCE(attention_changed_at, updated_at) DESC, updated_at DESC, created_at DESC, id DESC);

CREATE INDEX idx_sessions_catalog_activity
			ON sessions(
				workspace_id, state, COALESCE(last_update_at, updated_at) DESC,
				updated_at DESC, created_at DESC, id DESC
			);

CREATE INDEX idx_sessions_profile_catalog_activity
			ON sessions(
				profile_id, workspace_id, state, COALESCE(last_update_at, updated_at) DESC,
				updated_at DESC, created_at DESC, id DESC
			);

CREATE INDEX idx_sessions_catalog_recent
			ON sessions(workspace_id, state, updated_at DESC, created_at DESC, id DESC);

CREATE INDEX idx_sessions_profile_catalog_recent
			ON sessions(profile_id, workspace_id, state, updated_at DESC, created_at DESC, id DESC);

CREATE INDEX idx_sessions_global_catalog_navigator
 ON sessions(archived_at, (CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0
 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC);

CREATE INDEX idx_sessions_profile_global_catalog_navigator
 ON sessions(profile_id, archived_at, (CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0
 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC);

CREATE INDEX idx_sessions_workspace_catalog_navigator
 ON sessions(workspace_id, archived_at, (CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0
 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC);

CREATE INDEX idx_sessions_profile_workspace_catalog_navigator
 ON sessions(profile_id, workspace_id, archived_at, (CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0
 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC);

CREATE INDEX idx_sessions_catalog_created
 ON sessions(workspace_id, archived_at, created_at DESC, id DESC);

CREATE INDEX idx_sessions_profile_catalog_created
 ON sessions(profile_id, workspace_id, archived_at, created_at DESC, id DESC);

CREATE INDEX idx_sessions_catalog_archive_recent
			ON sessions(
				workspace_id, archived_at, state, updated_at DESC, created_at DESC, id DESC
			);

CREATE INDEX idx_sessions_profile_catalog_archive_recent
			ON sessions(
				profile_id, workspace_id, archived_at, state, updated_at DESC, created_at DESC, id DESC
			);

CREATE INDEX idx_sessions_parent ON sessions(parent_session_id);

CREATE INDEX idx_sessions_resumable
			ON sessions(state, failure_kind, last_update_at, updated_at);

CREATE INDEX idx_sessions_root ON sessions(root_session_id);

CREATE INDEX idx_sessions_soul_snapshot
			ON sessions(soul_snapshot_id);

CREATE INDEX idx_sessions_spawn_role ON sessions(spawn_role);

CREATE INDEX idx_sessions_type_depth ON sessions(session_type, spawn_depth);

CREATE INDEX idx_sessions_worktree ON sessions(worktree_id) WHERE worktree_id IS NOT NULL;

CREATE INDEX idx_session_pending_interactions_session_status
		ON session_pending_interactions(session_id, status, created_at, interaction_id);

CREATE UNIQUE INDEX uq_session_pending_interactions_active_provider_request
		ON session_pending_interactions(session_id, kind, provider_request_id)
		WHERE status IN ('pending', 'orphaned');

CREATE TRIGGER trg_sessions_archive_insert_guard
			BEFORE INSERT ON sessions
			WHEN NEW.archived_at IS NOT NULL AND NEW.state != 'stopped'
			BEGIN
				SELECT RAISE(ABORT, 'session is archived');
			END;

CREATE TRIGGER trg_sessions_archive_update_guard
			BEFORE UPDATE OF state, archived_at ON sessions
			WHEN NEW.archived_at IS NOT NULL AND NEW.state != 'stopped'
			BEGIN
				SELECT RAISE(ABORT, 'session is archived');
			END;

CREATE INDEX idx_token_stats_session ON token_stats(session_id);

CREATE UNIQUE INDEX idx_token_stats_session_agent ON token_stats(session_id, agent_name);

CREATE INDEX idx_token_usage_daily_workspace ON token_usage_daily(workspace_id, day);

CREATE UNIQUE INDEX uq_session_input_queue_active_steer
			ON session_input_queue(session_id)
			WHERE mode = 'steer' AND status = 'queued';

CREATE UNIQUE INDEX uq_session_input_queue_goal_prompt
			ON session_input_queue(loop_run_id, prompt_id)
			WHERE prompt_id IS NOT NULL;

CREATE UNIQUE INDEX uq_session_input_queue_synthetic_message
 ON session_input_queue(session_id, message_id)
 WHERE owner_kind = 'synthetic' AND message_id <> '' AND status IN ('queued','dispatching');

CREATE TABLE session_subagent_wakes (
 wake_message_id TEXT PRIMARY KEY,
 workspace_id TEXT NOT NULL,
 parent_session_id TEXT NOT NULL,
 state TEXT NOT NULL CHECK (state IN ('open','dispatched','settled','canceled')),
 route TEXT NOT NULL CHECK (route IN ('queue','steer')),
 input_entry_id TEXT NOT NULL DEFAULT '',
 steer_requeued INTEGER NOT NULL DEFAULT 0,
 attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 FOREIGN KEY (workspace_id, parent_session_id) REFERENCES sessions(workspace_id,id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX uq_session_subagent_wakes_open ON session_subagent_wakes(parent_session_id) WHERE state = 'open';

CREATE TABLE session_subagents (
 id TEXT PRIMARY KEY,
 workspace_id TEXT NOT NULL,
 parent_session_id TEXT NOT NULL,
 parent_turn_id TEXT NOT NULL,
 parent_tool_call_id TEXT NOT NULL DEFAULT '',
 child_session_id TEXT UNIQUE REFERENCES sessions(id) ON DELETE SET NULL,
 origin TEXT NOT NULL CHECK (origin IN ('delegated','provider_native')),
 provider_tool_call_id TEXT NOT NULL DEFAULT '',
 idempotency_key TEXT NOT NULL,
 request_fingerprint TEXT NOT NULL,
 title TEXT NOT NULL CHECK (length(title) <= 512),
 role TEXT NOT NULL DEFAULT 'general',
 task_chars INTEGER NOT NULL,
 pending_task TEXT,
 runtime_agent TEXT NOT NULL DEFAULT '',
 runtime_provider TEXT NOT NULL DEFAULT '',
 runtime_model TEXT NOT NULL DEFAULT '',
 runtime_reasoning_effort TEXT NOT NULL DEFAULT '',
 runtime_speed TEXT NOT NULL DEFAULT '',
 depth INTEGER NOT NULL CHECK (depth >= 1),
 status TEXT NOT NULL CHECK (status IN ('queued','running','waiting','completed','failed','canceled','interrupted')),
 work_state TEXT NOT NULL CHECK (work_state IN ('working','waiting_for_children','result_available')),
 progress TEXT NOT NULL DEFAULT '' CHECK (length(progress) <= 280),
 result TEXT,
 result_truncated INTEGER NOT NULL DEFAULT 0,
 error TEXT,
 wake_policy TEXT NOT NULL CHECK (wake_policy IN ('always','settled_only')),
 delivery TEXT NOT NULL DEFAULT 'none' CHECK (delivery IN ('none','pending','claimed','delivered','acknowledged','disposed')),
 wake_message_id TEXT REFERENCES session_subagent_wakes(wake_message_id) ON DELETE SET NULL,
 acknowledged_turn_id TEXT NOT NULL DEFAULT '',
 started_at TEXT,
 settled_at TEXT,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 FOREIGN KEY (workspace_id,parent_session_id) REFERENCES sessions(workspace_id,id) ON DELETE CASCADE,
 UNIQUE (parent_session_id,idempotency_key)
);
CREATE INDEX idx_session_subagents_parent_created ON session_subagents(parent_session_id,created_at);
CREATE INDEX idx_session_subagents_workspace_status ON session_subagents(workspace_id,status);
CREATE INDEX idx_session_subagents_parent_delivery ON session_subagents(parent_session_id,delivery) WHERE delivery IN ('pending','claimed');
CREATE INDEX idx_session_subagents_child ON session_subagents(child_session_id);
CREATE INDEX idx_session_subagents_provider_tool ON session_subagents(parent_session_id,provider_tool_call_id) WHERE origin = 'provider_native';
