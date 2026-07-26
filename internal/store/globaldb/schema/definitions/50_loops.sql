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
				)));

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

CREATE UNIQUE INDEX uq_loop_runs_active_session_goal ON loop_runs(origin_session_id)
			WHERE origin_kind='session'
			  AND status IN ('queued','running','watching','needs-approval','paused');

CREATE UNIQUE INDEX uq_loop_session_bindings_active
			ON loop_session_bindings(loop_run_id, handle) WHERE state='active';
