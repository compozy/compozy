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
		origin_json TEXT,
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
		mentions_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(mentions_json)),
		human_ingress INTEGER NOT NULL DEFAULT 0,
		app_ingress INTEGER NOT NULL DEFAULT 1,
		FOREIGN KEY (workspace_id, session_id) REFERENCES sessions(workspace_id, id) ON DELETE CASCADE,
		UNIQUE (workspace_id, session_id, idempotency_key),
		UNIQUE (workspace_id, session_id, message_id)
	);

	CREATE TABLE session_input_queue (
			goal_input_batch_max_rowid INTEGER CHECK (goal_input_batch_max_rowid IS NULL OR goal_input_batch_max_rowid >= 0),
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
			superseded_by TEXT,
			steer_delivery TEXT CHECK (steer_delivery IN ('injected', 'pending_injection', 'interrupt_fallback', 'pending_boundary', 'after_turn')),
			text TEXT NOT NULL,
			synthetic_prompt_json TEXT CHECK (synthetic_prompt_json IS NULL OR json_valid(synthetic_prompt_json)),
			skill_invocations_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(skill_invocations_json)),
			attachments_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(attachments_json)),
		origin_json TEXT,
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
				CHECK (terminal_tokens_used IS NULL OR terminal_tokens_used >= 0), terminal_at TIMESTAMP, priority INTEGER NOT NULL DEFAULT 0,
			origin_class TEXT NOT NULL DEFAULT 'human' CHECK (origin_class IN ('human', 'agent', 'synthetic')),
			origin_session_id TEXT NOT NULL DEFAULT '',
			mentions_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(mentions_json)),
			human_ingress INTEGER NOT NULL DEFAULT 0,
			app_ingress INTEGER NOT NULL DEFAULT 1,
			unattended_counted INTEGER NOT NULL DEFAULT 0);

CREATE INDEX idx_session_input_queue_generation
			ON session_input_queue(session_id, session_generation, status);

CREATE INDEX idx_session_input_queue_goal_owner
			ON session_input_queue(loop_run_id, task_run_id, owner_epoch, status, dispatchable, fence_kind);

	CREATE INDEX idx_session_input_queue_pending
			ON session_input_queue(session_id, status, origin_class, delivery DESC, priority DESC, enqueued_at ASC, id ASC);

	CREATE UNIQUE INDEX uq_session_input_queue_prompt_admission
			ON session_input_queue(prompt_admission_id)
			WHERE prompt_admission_id IS NOT NULL;

	CREATE INDEX idx_session_prompt_admissions_state
			ON session_prompt_admissions(workspace_id, session_id, state, updated_at);

	CREATE INDEX idx_session_prompt_admissions_human_turn
			ON session_prompt_admissions(session_id, turn_id)
			WHERE human_ingress = 1 AND dispatch_committed_at IS NOT NULL;

CREATE UNIQUE INDEX uq_session_input_queue_active_steer
			ON session_input_queue(session_id, origin_session_id)
			WHERE mode = 'steer' AND status = 'queued';

CREATE UNIQUE INDEX uq_session_input_queue_goal_prompt
			ON session_input_queue(loop_run_id, prompt_id)
			WHERE prompt_id IS NOT NULL;

CREATE UNIQUE INDEX uq_session_input_queue_synthetic_message
 ON session_input_queue(session_id, message_id)
 WHERE owner_kind = 'synthetic' AND message_id <> '' AND status IN ('queued','dispatching');
