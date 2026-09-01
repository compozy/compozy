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
	criteria_json     TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(criteria_json)),
	warnings_json     TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(warnings_json)),
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
			watch_seq    INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
			id           TEXT NOT NULL UNIQUE,
			loop_run_id  TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			seq          INTEGER NOT NULL,
			kind         TEXT NOT NULL CHECK (kind IN (
				'node_running','node_succeeded','node_failed','node_quarantined','node_requeued',
				'node_paused','node_resumed','node_wait_started','node_wait_resumed',
				'duplicate_suppressed','node_canceled','node_attention_flagged',
				'node_attention_cleared','target_breaker_transition','gate_verdict',
				'generation_started','channel_msg','token_tick','needs_approval','status_changed',
				'goal_turn_started','goal_turn_completed','goal_status_changed','runtime_applied',
				'predicate_diagnostic','route_taken','node_retry_scheduled','stale_schedule_dropped',
				'late_arrival','effect_results','custom_event','request_opened','request_answered',
				'request_expired','request_canceled','node_amended','branch_pruned','run_forked'
			)),
			payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
			at           TIMESTAMP NOT NULL,
			delivery_key TEXT
		);

CREATE TABLE loop_runs (
			id                   TEXT PRIMARY KEY,
			profile_id           TEXT NOT NULL REFERENCES profiles(id) ON DELETE RESTRICT,
			workspace_id         TEXT NOT NULL,
			loop_name            TEXT NOT NULL,
			status               TEXT NOT NULL,
			historical           INTEGER NOT NULL DEFAULT 0 CHECK (historical IN (0, 1)),
			completion_state     TEXT NOT NULL DEFAULT 'complete'
				CHECK (completion_state IN ('complete','partial')),
			forked_from_run_id   TEXT,
			forked_from_generation INTEGER,
			generation           INTEGER NOT NULL DEFAULT 0,
			reattempt_strategy   TEXT NOT NULL DEFAULT 'failed_only',
			last_progress_at     TIMESTAMP NOT NULL,
			completed_at         TIMESTAMP,
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
				CHECK (origin_creation_digest IS NULL OR length(trim(origin_creation_digest)) > 0), network_spec_json TEXT NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}'
				CHECK (json_valid(network_spec_json)), network_mode TEXT NOT NULL DEFAULT 'local'
				CHECK (network_mode IN ('local', 'live')), network_channel TEXT, network_source TEXT NOT NULL DEFAULT 'built_in_local'
					CHECK (network_source IN (
						'explicit_request', 'task_profile', 'workspace_coordination',
						'loop_definition', 'automation_job', 'built_in_local'
					)), best_generation INTEGER, best_score REAL,
					CHECK (
						(best_generation IS NULL AND best_score IS NULL)
						OR (best_generation IS NOT NULL AND best_score IS NOT NULL
							AND best_generation >= 1 AND best_generation <= generation)
					),
					CHECK (
						(forked_from_run_id IS NULL AND forked_from_generation IS NULL)
						OR (
							forked_from_run_id IS NOT NULL
							AND forked_from_generation IS NOT NULL
							AND length(trim(forked_from_run_id)) > 0
							AND forked_from_generation >= 1
						)
					));

CREATE TABLE loop_session_bindings (
	loop_run_id       TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	handle            TEXT NOT NULL CHECK (length(trim(handle)) > 0),
	binding_epoch     INTEGER NOT NULL CHECK (binding_epoch >= 1),
	binding_attempt_id TEXT NOT NULL UNIQUE CHECK (length(trim(binding_attempt_id)) > 0),
	session_id        TEXT NOT NULL CHECK (length(trim(session_id)) > 0),
	workspace_id      TEXT NOT NULL,
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

CREATE INDEX idx_loop_gate_decisions_workspace_run
			ON loop_gate_decisions(workspace_id, loop_run_id, generation, gate_id);

CREATE INDEX idx_loop_gate_verdicts_route_cause
			ON loop_gate_verdicts(loop_run_id, generation, route_cause_rank)
			WHERE route_cause_rank IS NOT NULL;

CREATE INDEX idx_loop_generation_outputs_output_ref
			ON loop_generation_outputs(output_ref);

CREATE INDEX idx_loop_node_controls_quarantined
	ON loop_node_controls(quarantined) WHERE quarantined = 1;

CREATE INDEX idx_loop_node_controls_attention
	ON loop_node_controls(attention_flag) WHERE attention_flag != '';

CREATE INDEX idx_loop_node_waits_due
	ON loop_node_waits(resume_at) WHERE claim_state = 'waiting' AND resume_at IS NOT NULL;

CREATE INDEX idx_loop_node_waits_ladder
	ON loop_node_waits(next_escalation_at)
	WHERE claim_state = 'waiting' AND next_escalation_at IS NOT NULL;

CREATE INDEX idx_loop_node_waits_state ON loop_node_waits(claim_state);

CREATE INDEX idx_loop_requests_pending
ON loop_requests(workspace_id, state, expires_at, opened_at);

CREATE UNIQUE INDEX uq_loop_timetravel_ops_idempotency
	ON loop_timetravel_ops(workspace_id, idempotency_key)
	WHERE idempotency_key != '';

CREATE INDEX idx_loop_effect_outbox_pending
	ON loop_effect_outbox(state) WHERE state = 'pending';

CREATE INDEX idx_loop_admission_claims_expiry
	ON loop_admission_claims(expires_at);

CREATE INDEX idx_loop_session_cleanup_pending
			ON loop_session_cleanup(id) WHERE completed_at IS NULL;

CREATE INDEX idx_loop_goal_session_outbox_pending
			ON loop_goal_session_outbox(id) WHERE delivered_at IS NULL;

CREATE INDEX idx_loop_run_events_run_seq
			ON loop_run_events(loop_run_id, seq);

CREATE INDEX idx_loop_run_events_watch_stream
	ON loop_run_events(workspace_id, watch_seq);

CREATE UNIQUE INDEX uq_loop_run_events_delivery
	ON loop_run_events(loop_run_id, delivery_key) WHERE delivery_key IS NOT NULL;

CREATE INDEX idx_loop_runs_catalog
			ON loop_runs(workspace_id, loop_name, created_at DESC, id DESC, status);

CREATE INDEX idx_loop_runs_queue_order
			ON loop_runs(workspace_id, loop_name, status, created_at ASC, id ASC);

CREATE UNIQUE INDEX uq_loop_runs_active_session_goal ON loop_runs(origin_session_id)
			WHERE origin_kind='session'
			  AND historical = 0
			  AND status IN ('queued','running','watching','needs-approval','paused');

CREATE UNIQUE INDEX uq_loop_session_bindings_active
			ON loop_session_bindings(loop_run_id, handle) WHERE state='active';
