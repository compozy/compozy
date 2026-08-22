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
			runtime_defaults_json TEXT CHECK (runtime_defaults_json IS NULL OR json_valid(runtime_defaults_json)),
			runtime_rules_json    TEXT CHECK (runtime_rules_json IS NULL OR json_valid(runtime_rules_json)),
			environment_json      TEXT CHECK (environment_json IS NULL OR json_valid(environment_json)),
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

CREATE TABLE loop_gate_verdicts (
			loop_run_id          TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
			generation           INTEGER NOT NULL CHECK (generation >= 1),
			gate_id              TEXT NOT NULL,
			item_index           INTEGER NOT NULL DEFAULT 0 CHECK (item_index >= 0),
			outcome              TEXT NOT NULL CHECK (outcome IN (
				'approved','rejected','awaiting_approval','blocked','error','timeout','invalid_output'
			)),
			score                REAL,
			route_cause_rank     INTEGER,
			blocking_issues_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(blocking_issues_json)),
			criteria_json        TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(criteria_json)),
			decided_at           TIMESTAMP NOT NULL,
			PRIMARY KEY (loop_run_id, generation, gate_id, item_index)
		);

CREATE TABLE loop_generations (
			loop_run_id       TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
			generation        INTEGER NOT NULL CHECK (generation >= 1),
			parent_generation INTEGER NOT NULL DEFAULT 0
				CHECK (parent_generation >= 0 AND parent_generation < generation),
			origin            TEXT NOT NULL CHECK (origin IN (
				'initial','stop_when','reattempt','gate_revise','gate_next_generation',
				'dod_retry','ratchet_restore','requeue','operator_rerun','fork_seed'
			)),
			created_at        TIMESTAMP NOT NULL,
			PRIMARY KEY (loop_run_id, generation)
		);

CREATE TABLE loop_generation_outputs (
			loop_run_id       TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
			generation        INTEGER NOT NULL,
			node_id           TEXT NOT NULL,
			item_index        INTEGER NOT NULL DEFAULT 0,
			output_id         TEXT,
			artifact_name     TEXT,
			status            TEXT NOT NULL CHECK (status IN (
				'pending','enqueued','running','retrying','waiting','paused','awaiting_child',
				'control_pending','awaiting_goal','succeeded','partial','failed','canceled','quarantined'
			)),
			output_ref        TEXT,
			task_run_id       TEXT,
			child_loop_run_id TEXT,
			resolved_runtime_json TEXT CHECK (resolved_runtime_json IS NULL OR json_valid(resolved_runtime_json)),
			attempt INTEGER NOT NULL DEFAULT 1 CHECK (attempt >= 1),
			next_attempt_at TIMESTAMP,
			first_scheduled_at TIMESTAMP,
			epoch INTEGER NOT NULL DEFAULT 0,
			goal_status TEXT, goal_turns_used INTEGER, goal_turn_limit INTEGER,
			PRIMARY KEY (loop_run_id, generation, node_id, item_index)
		);

CREATE INDEX idx_loop_generation_outputs_retry_due
	ON loop_generation_outputs(next_attempt_at, loop_run_id, generation, node_id, item_index)
	WHERE status = 'retrying' AND next_attempt_at IS NOT NULL;

CREATE TABLE loop_node_controls (
	loop_run_id           TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	node_id               TEXT NOT NULL,
	paused                INTEGER NOT NULL DEFAULT 0,
	pause_actor_kind      TEXT,
	pause_actor_id        TEXT,
	pause_reason          TEXT,
	pause_rule_id         TEXT,
	pause_requested_at    TIMESTAMP,
	quarantined           INTEGER NOT NULL DEFAULT 0,
	quarantine_entry_json TEXT CHECK (quarantine_entry_json IS NULL OR json_valid(quarantine_entry_json)),
	quarantined_at        TIMESTAMP,
	attention_flag        TEXT NOT NULL DEFAULT '' CHECK (attention_flag IN (
		'', 'silence', 'resume_exhausted', 'dependency_quarantined', 'wait_intervention', 'expired_wait'
	)),
	attention_reason      TEXT NOT NULL DEFAULT '',
	attention_producer_node_id TEXT NOT NULL DEFAULT '',
	cancel_state          TEXT NOT NULL DEFAULT '' CHECK (cancel_state IN (
		'', 'requested', 'delivering', 'draining', 'canceled'
	)),
	cancel_actor_kind     TEXT,
	cancel_actor_id       TEXT,
	cancel_reason         TEXT,
	cancel_requested_at   TIMESTAMP,
	last_evidence_at      TIMESTAMP,
	death_resume_streak   INTEGER NOT NULL DEFAULT 0 CHECK (death_resume_streak >= 0),
	gate_revisions_json   TEXT NOT NULL DEFAULT '{}' CHECK (
		json_valid(gate_revisions_json) AND json_type(gate_revisions_json) = 'object'
	),
	revision              INTEGER NOT NULL DEFAULT 0,
	updated_at            TIMESTAMP NOT NULL,
	PRIMARY KEY (loop_run_id, node_id)
);

CREATE TABLE loop_node_attempts (
	loop_run_id    TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	generation     INTEGER NOT NULL CHECK (generation >= 1),
	node_id        TEXT NOT NULL,
	item_index     INTEGER NOT NULL DEFAULT 0,
	attempt        INTEGER NOT NULL CHECK (attempt >= 1),
	failure_class  TEXT CHECK (failure_class IS NULL OR failure_class IN (
		'transport','payload_declared','quality_rejection','authoring','cancellation',
		'attempt_timeout','budget_exhausted','target_unavailable'
	)),
	failure_code   TEXT NOT NULL DEFAULT '',
	cause          TEXT NOT NULL DEFAULT '',
	hint           TEXT NOT NULL DEFAULT '',
	target         TEXT NOT NULL DEFAULT '',
	disposition    TEXT NOT NULL CHECK (disposition IN (
		'succeeded','retried','routed','absorbed','escalated','quarantined','canceled','resumed'
	)),
	started_at     TIMESTAMP NOT NULL,
	ended_at       TIMESTAMP,
	next_attempt_at TIMESTAMP,
	PRIMARY KEY (loop_run_id, generation, node_id, item_index, attempt)
);

CREATE TABLE loop_node_waits (
	loop_run_id        TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	generation         INTEGER NOT NULL CHECK (generation >= 1),
	node_id            TEXT NOT NULL,
	item_index         INTEGER NOT NULL DEFAULT 0,
	kind               TEXT NOT NULL CHECK (kind IN ('timer','event','approval_escalation','request')),
	resume_at          TIMESTAMP,
	next_escalation_at TIMESTAMP,
	escalation_cursor  INTEGER NOT NULL DEFAULT 0,
	claim_state        TEXT NOT NULL DEFAULT 'waiting' CHECK (claim_state IN (
		'waiting','claimed','resumed','intervention_required'
	)),
	claimed_by_kind    TEXT,
	claimed_by_id      TEXT,
	claimed_at         TIMESTAMP,
	admission_failures INTEGER NOT NULL DEFAULT 0,
	expect_json        TEXT CHECK (expect_json IS NULL OR json_valid(expect_json)),
	ahead_payload_json TEXT CHECK (ahead_payload_json IS NULL OR json_valid(ahead_payload_json)),
	issued_epoch       INTEGER NOT NULL DEFAULT 0,
	created_at         TIMESTAMP NOT NULL,
	PRIMARY KEY (loop_run_id, generation, node_id, item_index)
);

CREATE TABLE loop_requests (
	workspace_id          TEXT NOT NULL,
	loop_run_id           TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	generation            INTEGER NOT NULL CHECK (generation >= 1),
	node_id               TEXT NOT NULL,
	item_index            INTEGER NOT NULL DEFAULT 0 CHECK (item_index >= 0),
	kind                  TEXT NOT NULL CHECK (kind IN ('ask','review')),
	state                 TEXT NOT NULL CHECK (state IN ('pending','answered','expired','canceled')),
	prompt                TEXT NOT NULL,
	context_preview_json  TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(context_preview_json)),
	context_ref           TEXT,
	answer_schema_json    TEXT CHECK (answer_schema_json IS NULL OR json_valid(answer_schema_json)),
	edit_schema_json      TEXT CHECK (edit_schema_json IS NULL OR json_valid(edit_schema_json)),
	respond_schema_json   TEXT CHECK (respond_schema_json IS NULL OR json_valid(respond_schema_json)),
	decisions_json        TEXT NOT NULL CHECK (json_valid(decisions_json)),
	proposed_ref          TEXT,
	proposed_preview_json TEXT CHECK (proposed_preview_json IS NULL OR json_valid(proposed_preview_json)),
	answered_decision     TEXT,
	answered_payload_ref  TEXT,
	answered_note         TEXT,
	actor_kind            TEXT,
	actor_id              TEXT,
	opened_at             TIMESTAMP NOT NULL,
	resolved_at           TIMESTAMP,
	expires_at            TIMESTAMP,
	PRIMARY KEY (loop_run_id, generation, node_id, item_index)
);

CREATE TABLE loop_timetravel_ops (
	workspace_id      TEXT NOT NULL,
	op_id             TEXT NOT NULL,
	kind              TEXT NOT NULL CHECK (kind IN ('rerun','fork')),
	idempotency_key   TEXT NOT NULL DEFAULT '',
	request_digest    TEXT NOT NULL CHECK (length(request_digest) = 64),
	source_run_id     TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	source_generation INTEGER CHECK (source_generation IS NULL OR source_generation >= 1),
	from_node         TEXT,
	item_index        INTEGER CHECK (item_index IS NULL OR item_index >= 0),
	actor_kind        TEXT NOT NULL,
	actor_id          TEXT NOT NULL,
	reason            TEXT,
	result_run_id     TEXT NOT NULL,
	result_generation INTEGER CHECK (result_generation IS NULL OR result_generation >= 1),
	created_at        TIMESTAMP NOT NULL,
	PRIMARY KEY (workspace_id, op_id),
	CHECK (
		(kind = 'rerun' AND source_generation IS NOT NULL AND from_node IS NOT NULL
		 AND result_generation IS NOT NULL AND result_run_id = source_run_id)
		OR
		(kind = 'fork' AND source_generation IS NOT NULL AND from_node IS NULL
		 AND item_index IS NULL AND result_generation IS NULL)
	)
);

CREATE TABLE loop_node_amendments (
	workspace_id  TEXT NOT NULL,
	loop_run_id   TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	generation    INTEGER NOT NULL CHECK (generation >= 1),
	node_id       TEXT NOT NULL,
	item_index    INTEGER NOT NULL DEFAULT 0 CHECK (item_index >= 0),
	amendment_seq INTEGER NOT NULL CHECK (amendment_seq >= 1),
	original_ref  TEXT NOT NULL,
	amended_ref   TEXT NOT NULL,
	actor_kind    TEXT NOT NULL,
	actor_id      TEXT NOT NULL,
	reason        TEXT,
	created_at    TIMESTAMP NOT NULL,
	PRIMARY KEY (loop_run_id, generation, node_id, item_index, amendment_seq)
);

CREATE TABLE loop_node_lane_pauses (
	workspace_id TEXT NOT NULL,
	loop_run_id  TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	generation   INTEGER NOT NULL CHECK (generation >= 1),
	node_id      TEXT NOT NULL,
	item_index   INTEGER NOT NULL CHECK (item_index >= 0),
	actor_kind   TEXT NOT NULL,
	actor_id     TEXT NOT NULL,
	reason       TEXT,
	mode         TEXT NOT NULL CHECK (mode IN ('drain','cancel')),
	requested_at TIMESTAMP NOT NULL,
	PRIMARY KEY (loop_run_id, generation, node_id, item_index)
);

CREATE TABLE loop_admission_claims (
	workspace_id       TEXT NOT NULL,
	loop_name          TEXT NOT NULL,
	source_key         TEXT NOT NULL,
	event_key          TEXT NOT NULL,
	loop_run_id        TEXT NOT NULL,
	claimed_at         TIMESTAMP NOT NULL,
	expires_at         TIMESTAMP NOT NULL,
	suppressed_count   INTEGER NOT NULL DEFAULT 0,
	last_suppressed_at TIMESTAMP,
	PRIMARY KEY (workspace_id, loop_name, source_key, event_key)
);

CREATE TABLE loop_effect_outbox (
	loop_run_id     TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	delivery_id     TEXT NOT NULL,
	source_event_id TEXT NOT NULL,
	trigger         TEXT NOT NULL,
	generation      INTEGER NOT NULL,
	node_id         TEXT NOT NULL DEFAULT '',
	item_index      INTEGER NOT NULL DEFAULT 0,
	entry_index     INTEGER NOT NULL CHECK (entry_index >= 0),
	entry_json      TEXT NOT NULL CHECK (json_valid(entry_json)),
	state           TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','delivered','failed')),
	attempts        INTEGER NOT NULL DEFAULT 0,
	created_at      TIMESTAMP NOT NULL,
	delivered_at    TIMESTAMP,
	PRIMARY KEY (loop_run_id, delivery_id)
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
	criteria_json     TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(criteria_json)),
	warnings_json     TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(warnings_json)),
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
				'duplicate_suppressed','node_canceled','node_killed','node_attention_flagged',
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
					cancel_requested INTEGER NOT NULL DEFAULT 0,
					cancel_kind TEXT NOT NULL DEFAULT '' CHECK (cancel_kind IN ('', 'cancel', 'kill')),
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

CREATE INDEX idx_loop_goal_session_cleanup_pending
			ON loop_goal_session_cleanup(id) WHERE completed_at IS NULL;

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
