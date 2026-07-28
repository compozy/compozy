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
				CHECK (creation_profile_ref IS NULL OR length(trim(creation_profile_ref)) > 0), network_spec_json TEXT NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}'
				CHECK (json_valid(network_spec_json)), network_mode TEXT NOT NULL DEFAULT 'local'
				CHECK (network_mode IN ('local', 'live')), network_channel TEXT, network_source TEXT NOT NULL DEFAULT 'built_in_local'
				CHECK (network_source IN (
					'explicit_request', 'task_profile', 'workspace_coordination',
					'loop_definition', 'automation_job', 'built_in_local'
				)),
		UNIQUE (workspace_id, id));

CREATE TABLE token_stats (
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
		PRIMARY KEY (day, workspace_id, agent_name)
	);

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

CREATE INDEX idx_token_stats_session ON token_stats(session_id);

CREATE UNIQUE INDEX idx_token_stats_session_agent ON token_stats(session_id, agent_name);

CREATE INDEX idx_token_usage_daily_workspace ON token_usage_daily(workspace_id, day);

CREATE UNIQUE INDEX uq_session_input_queue_active_steer
			ON session_input_queue(session_id)
			WHERE mode = 'steer' AND status IN ('queued', 'dispatching');

CREATE UNIQUE INDEX uq_session_input_queue_goal_prompt
			ON session_input_queue(loop_run_id, prompt_id)
			WHERE prompt_id IS NOT NULL;
