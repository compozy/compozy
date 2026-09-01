-- +goose Up
PRAGMA foreign_keys = off;

DROP TRIGGER workspace_scope_cleanup_after_delete;
DROP TRIGGER automation_watch_events_after_insert;
DROP TRIGGER automation_watch_events_after_terminal_update;

CREATE TABLE loop_session_cleanup (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	cleanup_id TEXT NOT NULL UNIQUE CHECK (length(trim(cleanup_id)) > 0),
	workspace_id TEXT NOT NULL,
	loop_run_id TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	source_kind TEXT NOT NULL CHECK (source_kind IN ('goal-binding','task-run')),
	source_id TEXT NOT NULL CHECK (length(trim(source_id)) > 0),
	source_epoch INTEGER NOT NULL CHECK (source_epoch >= 0),
	session_id TEXT NOT NULL CHECK (length(trim(session_id)) > 0),
	cause TEXT NOT NULL CHECK (cause IN ('terminal','reseed','control-revoked','stop','operator-cancel')),
	created_at TIMESTAMP NOT NULL,
	completed_at TIMESTAMP CHECK (completed_at IS NULL OR completed_at >= created_at),
	UNIQUE (loop_run_id, source_kind, source_id, source_epoch),
	CHECK (
		(source_kind = 'goal-binding' AND source_epoch >= 1)
		OR (source_kind = 'task-run' AND source_epoch = 0)
	)
);

INSERT INTO loop_session_cleanup (
	id, cleanup_id, workspace_id, loop_run_id, source_kind, source_id, source_epoch,
	session_id, cause, created_at, completed_at
)
SELECT id,
	'loop-session-cleanup:' || loop_run_id || ':goal-binding:' || handle || ':' || binding_epoch,
	workspace_id, loop_run_id, 'goal-binding', handle, binding_epoch,
	session_id, cause, created_at, completed_at
FROM loop_goal_session_cleanup;

INSERT OR IGNORE INTO loop_session_cleanup (
	cleanup_id, workspace_id, loop_run_id, source_kind, source_id, source_epoch,
	session_id, cause, created_at
)
SELECT 'loop-session-cleanup:' || binding.loop_run_id || ':goal-binding:' ||
		binding.handle || ':' || binding.binding_epoch,
	binding.workspace_id, binding.loop_run_id, 'goal-binding', binding.handle, binding.binding_epoch,
	binding.session_id, 'operator-cancel',
	COALESCE(run.control_requested_at, run.last_progress_at)
FROM loop_session_bindings AS binding
JOIN loop_runs AS run ON run.id = binding.loop_run_id
WHERE run.cancel_requested = 1
	AND binding.ownership = 'run-owned'
	AND binding.state IN ('creating','active');

INSERT OR IGNORE INTO loop_session_cleanup (
	cleanup_id, workspace_id, loop_run_id, source_kind, source_id, source_epoch,
	session_id, cause, created_at
)
SELECT 'loop-session-cleanup:' || task_run.loop_run_id || ':task-run:' || task_run.id || ':0',
	task_run.workspace_id, task_run.loop_run_id, 'task-run', task_run.id, 0,
	task_run.session_id, 'operator-cancel',
	COALESCE(run.control_requested_at, run.last_progress_at)
FROM task_runs AS task_run
JOIN loop_runs AS run ON run.id = task_run.loop_run_id
WHERE run.cancel_requested = 1
	AND task_run.run_kind = 'worker'
	AND length(trim(COALESCE(task_run.session_id, ''))) > 0
	AND task_run.session_id != COALESCE(run.origin_session_id, '');

UPDATE loop_generation_outputs
SET status = 'canceled', next_attempt_at = NULL
WHERE loop_run_id IN (SELECT id FROM loop_runs WHERE cancel_requested = 1)
	AND status IN (
		'pending','enqueued','running','retrying','waiting','paused','awaiting_child',
		'control_pending','awaiting_goal'
	);

UPDATE task_runs
SET status = 'canceled',
	ended_at = COALESCE(ended_at, (
		SELECT COALESCE(control_requested_at, last_progress_at)
		FROM loop_runs WHERE loop_runs.id = task_runs.loop_run_id
	)),
	error = 'Loop run reached a terminal state',
	claim_token = NULL,
	lease_until = NULL,
	heartbeat_at = NULL
WHERE loop_run_id IN (SELECT id FROM loop_runs WHERE cancel_requested = 1)
	AND status IN ('queued','claimed','starting','running','needs_attention');

UPDATE tasks
SET status = 'canceled',
	current_run_id = NULL,
	closed_at = COALESCE(closed_at, updated_at)
WHERE id IN (
	SELECT task_id FROM task_runs
	WHERE loop_run_id IN (SELECT id FROM loop_runs WHERE cancel_requested = 1)
		AND task_id IS NOT NULL
)
	AND status NOT IN ('completed','failed','canceled');

UPDATE loop_runs
SET status = 'canceled',
	completed_at = COALESCE(completed_at, control_requested_at, last_progress_at),
	pause_requested = 0
WHERE cancel_requested = 1
	AND status IN ('queued','running','watching','needs-approval','paused');

DROP TABLE loop_goal_session_cleanup;

CREATE TABLE new_loop_runs (
	id TEXT PRIMARY KEY,
	profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE RESTRICT,
	workspace_id TEXT NOT NULL,
	loop_name TEXT NOT NULL,
	status TEXT NOT NULL,
	historical INTEGER NOT NULL DEFAULT 0 CHECK (historical IN (0, 1)),
	completion_state TEXT NOT NULL DEFAULT 'complete' CHECK (completion_state IN ('complete','partial')),
	forked_from_run_id TEXT,
	forked_from_generation INTEGER,
	generation INTEGER NOT NULL DEFAULT 0,
	reattempt_strategy TEXT NOT NULL DEFAULT 'failed_only',
	last_progress_at TIMESTAMP NOT NULL,
	completed_at TIMESTAMP,
	budget_tokens INTEGER NOT NULL DEFAULT 0,
	budget_wall_sec INTEGER NOT NULL DEFAULT 0,
	budget_on_exceeded TEXT NOT NULL DEFAULT 'halt',
	tokens_used INTEGER NOT NULL DEFAULT 0,
	parent_loop_run_id TEXT,
	pause_requested INTEGER NOT NULL DEFAULT 0,
	inputs_json TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z',
	iteration_cap INTEGER NOT NULL DEFAULT 0,
	started_by_kind TEXT NOT NULL DEFAULT '',
	started_by_ref TEXT NOT NULL DEFAULT '',
	started_origin_kind TEXT NOT NULL DEFAULT '',
	started_origin_ref TEXT NOT NULL DEFAULT '',
	started_at TEXT NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z',
	definition_version INTEGER NOT NULL DEFAULT 0,
	definition_digest TEXT NOT NULL DEFAULT '',
	active_gate_id TEXT NOT NULL DEFAULT '',
	active_human_criteria_json TEXT NOT NULL DEFAULT '[]',
	budget_approval_seq INTEGER NOT NULL DEFAULT 0,
	start_metadata_json TEXT NOT NULL DEFAULT '{}',
	origin_kind TEXT NOT NULL DEFAULT 'catalog',
	origin_session_id TEXT,
	goal_cleared_at TIMESTAMP,
	budget_version INTEGER NOT NULL DEFAULT 0 CHECK (budget_version >= 0),
	goal_context_nudge_ratio REAL NOT NULL DEFAULT 0.8
		CHECK (goal_context_nudge_ratio >= 0.0 AND goal_context_nudge_ratio <= 1.0),
	control_actor_kind TEXT,
	control_actor_id TEXT,
	control_requested_at TIMESTAMP,
	origin_creation_profile_ref TEXT
		CHECK (origin_creation_profile_ref IS NULL OR length(trim(origin_creation_profile_ref)) > 0),
	origin_policy_spec_digest TEXT
		CHECK (origin_policy_spec_digest IS NULL OR length(trim(origin_policy_spec_digest)) > 0),
	origin_creation_digest TEXT
		CHECK (origin_creation_digest IS NULL OR length(trim(origin_creation_digest)) > 0),
	network_spec_json TEXT NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}'
		CHECK (json_valid(network_spec_json)),
	network_mode TEXT NOT NULL DEFAULT 'local' CHECK (network_mode IN ('local', 'live')),
	network_channel TEXT,
	network_source TEXT NOT NULL DEFAULT 'built_in_local' CHECK (network_source IN (
		'explicit_request', 'task_profile', 'workspace_coordination',
		'loop_definition', 'automation_job', 'built_in_local'
	)),
	best_generation INTEGER,
	best_score REAL,
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
	)
);

INSERT INTO new_loop_runs (
	id, profile_id, workspace_id, loop_name, status, historical, completion_state,
	forked_from_run_id, forked_from_generation, generation, reattempt_strategy,
	last_progress_at, completed_at, budget_tokens, budget_wall_sec, budget_on_exceeded,
	tokens_used, parent_loop_run_id, pause_requested, inputs_json, created_at, iteration_cap,
	started_by_kind, started_by_ref, started_origin_kind, started_origin_ref, started_at,
	definition_version, definition_digest, active_gate_id, active_human_criteria_json,
	budget_approval_seq, start_metadata_json, origin_kind, origin_session_id, goal_cleared_at,
	budget_version, goal_context_nudge_ratio, control_actor_kind, control_actor_id,
	control_requested_at, origin_creation_profile_ref, origin_policy_spec_digest,
	origin_creation_digest, network_spec_json, network_mode, network_channel, network_source,
	best_generation, best_score
)
SELECT id, profile_id, workspace_id, loop_name, status, historical, completion_state,
	forked_from_run_id, forked_from_generation, generation, reattempt_strategy,
	last_progress_at, completed_at, budget_tokens, budget_wall_sec, budget_on_exceeded,
	tokens_used, parent_loop_run_id, pause_requested, inputs_json, created_at, iteration_cap,
	started_by_kind, started_by_ref, started_origin_kind, started_origin_ref, started_at,
	definition_version, definition_digest, active_gate_id, active_human_criteria_json,
	budget_approval_seq, start_metadata_json, origin_kind, origin_session_id, goal_cleared_at,
	budget_version, goal_context_nudge_ratio, control_actor_kind, control_actor_id,
	control_requested_at, origin_creation_profile_ref, origin_policy_spec_digest,
	origin_creation_digest, network_spec_json, network_mode, network_channel, network_source,
	best_generation, best_score
FROM loop_runs;

DROP TABLE loop_runs;
ALTER TABLE new_loop_runs RENAME TO loop_runs;

CREATE TABLE new_loop_node_controls (
	loop_run_id TEXT NOT NULL REFERENCES loop_runs(id) ON DELETE CASCADE,
	node_id TEXT NOT NULL,
	paused INTEGER NOT NULL DEFAULT 0,
	pause_actor_kind TEXT,
	pause_actor_id TEXT,
	pause_reason TEXT,
	pause_rule_id TEXT,
	pause_requested_at TIMESTAMP,
	quarantined INTEGER NOT NULL DEFAULT 0,
	quarantine_entry_json TEXT CHECK (quarantine_entry_json IS NULL OR json_valid(quarantine_entry_json)),
	quarantined_at TIMESTAMP,
	attention_flag TEXT NOT NULL DEFAULT '' CHECK (attention_flag IN (
		'', 'silence', 'resume_exhausted', 'dependency_quarantined', 'wait_intervention', 'expired_wait'
	)),
	attention_reason TEXT NOT NULL DEFAULT '',
	attention_producer_node_id TEXT NOT NULL DEFAULT '',
	cancel_state TEXT NOT NULL DEFAULT '' CHECK (cancel_state IN ('', 'canceled')),
	cancel_actor_kind TEXT,
	cancel_actor_id TEXT,
	cancel_reason TEXT,
	cancel_requested_at TIMESTAMP,
	last_evidence_at TIMESTAMP,
	death_resume_streak INTEGER NOT NULL DEFAULT 0 CHECK (death_resume_streak >= 0),
	gate_revisions_json TEXT NOT NULL DEFAULT '{}' CHECK (
		json_valid(gate_revisions_json) AND json_type(gate_revisions_json) = 'object'
	),
	revision INTEGER NOT NULL DEFAULT 0,
	updated_at TIMESTAMP NOT NULL,
	PRIMARY KEY (loop_run_id, node_id)
);

INSERT INTO new_loop_node_controls (
	loop_run_id, node_id, paused, pause_actor_kind, pause_actor_id, pause_reason,
	pause_rule_id, pause_requested_at, quarantined, quarantine_entry_json, quarantined_at,
	attention_flag, attention_reason, attention_producer_node_id, cancel_state,
	cancel_actor_kind, cancel_actor_id, cancel_reason, cancel_requested_at, last_evidence_at,
	death_resume_streak, gate_revisions_json, revision, updated_at
)
SELECT loop_run_id, node_id, paused, pause_actor_kind, pause_actor_id, pause_reason,
	pause_rule_id, pause_requested_at, quarantined, quarantine_entry_json, quarantined_at,
	attention_flag, attention_reason, attention_producer_node_id,
	CASE WHEN cancel_state IN ('requested','delivering','draining') THEN 'canceled' ELSE cancel_state END,
	cancel_actor_kind, cancel_actor_id, cancel_reason, cancel_requested_at, last_evidence_at,
	death_resume_streak, gate_revisions_json, revision, updated_at
FROM loop_node_controls;

DROP TABLE loop_node_controls;
ALTER TABLE new_loop_node_controls RENAME TO loop_node_controls;

UPDATE loop_node_attempts
SET failure_code = 'operator_cancel'
WHERE failure_code = 'operator_kill';

UPDATE loop_node_attempts
SET cause = 'operator_cancel'
WHERE cause = 'operator_kill';

CREATE TABLE new_loop_run_events (
	watch_seq INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
	id TEXT NOT NULL UNIQUE,
	loop_run_id TEXT NOT NULL,
	workspace_id TEXT NOT NULL,
	seq INTEGER NOT NULL,
	kind TEXT NOT NULL CHECK (kind IN (
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
	at TIMESTAMP NOT NULL,
	delivery_key TEXT
);

INSERT INTO new_loop_run_events (
	watch_seq, id, loop_run_id, workspace_id, seq, kind, payload_json, at, delivery_key
)
SELECT watch_seq, id, loop_run_id, workspace_id, seq,
	CASE WHEN kind = 'node_killed' THEN 'node_canceled' ELSE kind END,
	CASE
		WHEN json_extract(payload_json, '$.cause') = 'operator_kill'
		THEN json_set(payload_json, '$.cause', 'operator_cancel')
		ELSE payload_json
	END,
	at, delivery_key
FROM loop_run_events;

DROP TABLE loop_run_events;
ALTER TABLE new_loop_run_events RENAME TO loop_run_events;

CREATE INDEX idx_loop_node_controls_quarantined
	ON loop_node_controls(quarantined) WHERE quarantined = 1;
CREATE INDEX idx_loop_node_controls_attention
	ON loop_node_controls(attention_flag) WHERE attention_flag != '';
CREATE INDEX idx_loop_session_cleanup_pending
	ON loop_session_cleanup(id) WHERE completed_at IS NULL;
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

-- +goose StatementBegin
CREATE TRIGGER loop_runs_profile_owner_immutable BEFORE UPDATE OF profile_id ON loop_runs
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER loop_runs_profile_owner_active BEFORE INSERT ON loop_runs BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER automation_watch_events_after_insert
AFTER INSERT ON automation_runs
WHEN NEW.status IN ('completed', 'failed')
BEGIN
	INSERT INTO automation_watch_events (
		run_id, job_id, trigger_id, session_id, status, attempt,
		started_at, ended_at, error, agent_name, workspace_id, retry_json
	) VALUES (
		NEW.id, COALESCE(NEW.job_id, ''), COALESCE(NEW.trigger_id, ''),
		COALESCE(NEW.session_id, ''), NEW.status, NEW.attempt, NEW.started_at, NEW.ended_at,
		COALESCE(NEW.error, ''),
		COALESCE(
			(SELECT agent_name FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT agent_name FROM automation_triggers WHERE id = NEW.trigger_id), ''
		),
		COALESCE(
			NULLIF((SELECT workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			NULLIF((SELECT workspace_id FROM loop_runs WHERE id = NEW.loop_run_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''), ''
		),
		COALESCE(
			(SELECT retry FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT retry FROM automation_triggers WHERE id = NEW.trigger_id), ''
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
		NEW.id, COALESCE(NEW.job_id, ''), COALESCE(NEW.trigger_id, ''),
		COALESCE(NEW.session_id, ''), NEW.status, NEW.attempt, NEW.started_at, NEW.ended_at,
		COALESCE(NEW.error, ''),
		COALESCE(
			(SELECT agent_name FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT agent_name FROM automation_triggers WHERE id = NEW.trigger_id), ''
		),
		COALESCE(
			NULLIF((SELECT workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			NULLIF((SELECT workspace_id FROM loop_runs WHERE id = NEW.loop_run_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''), ''
		),
		COALESCE(
			(SELECT retry FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT retry FROM automation_triggers WHERE id = NEW.trigger_id), ''
		)
	);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER workspace_scope_cleanup_after_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM network_wake_events WHERE workspace_id = OLD.id;
	DELETE FROM network_wake_sources WHERE workspace_id = OLD.id;
	DELETE FROM network_message_dispositions WHERE workspace_id = OLD.id;
	DELETE FROM network_live_wakes WHERE workspace_id = OLD.id;
	DELETE FROM network_participation_budgets WHERE workspace_id = OLD.id;
	DELETE FROM network_task_status_projections WHERE workspace_id = OLD.id;
	DELETE FROM network_task_thread_origins WHERE workspace_id = OLD.id;
	DELETE FROM network_thread_session_token_stats WHERE workspace_id = OLD.id;
	DELETE FROM network_thread_participants WHERE workspace_id = OLD.id;
	DELETE FROM network_subscriptions WHERE workspace_id = OLD.id;
	DELETE FROM network_work WHERE workspace_id = OLD.id;
	DELETE FROM network_direct_rooms WHERE workspace_id = OLD.id;
	DELETE FROM network_threads WHERE workspace_id = OLD.id;
	DELETE FROM network_channel_participants WHERE workspace_id = OLD.id;
	DELETE FROM network_channel_kind_counts WHERE workspace_id = OLD.id;
	DELETE FROM network_channel_stats WHERE workspace_id = OLD.id;
	DELETE FROM network_timeline_log WHERE workspace_id = OLD.id;
	DELETE FROM network_channels WHERE workspace_id = OLD.id;
	DELETE FROM network_audit_log WHERE workspace_id = OLD.id;
	DELETE FROM network_coordination_invitations WHERE workspace_id = OLD.id;
	DELETE FROM task_network_coordination WHERE workspace_id = OLD.id;
	DELETE FROM loop_ui_annotations WHERE workspace_id = OLD.id;
	DELETE FROM loop_session_bindings WHERE workspace_id = OLD.id;
	DELETE FROM loop_run_events WHERE workspace_id = OLD.id;
	DELETE FROM loop_runs WHERE workspace_id = OLD.id;
	DELETE FROM loop_goal_session_outbox WHERE workspace_id = OLD.id;
	DELETE FROM loop_session_cleanup WHERE workspace_id = OLD.id;
	DELETE FROM loop_admission_claims WHERE workspace_id = OLD.id;
	DELETE FROM loop_node_lane_pauses WHERE workspace_id = OLD.id;
	DELETE FROM loop_node_amendments WHERE workspace_id = OLD.id;
	DELETE FROM loop_timetravel_ops WHERE workspace_id = OLD.id;
	DELETE FROM loop_requests WHERE workspace_id = OLD.id;
	DELETE FROM loop_gate_decisions WHERE workspace_id = OLD.id;
	DELETE FROM loop_definition_snapshots WHERE workspace_id = OLD.id;
	DELETE FROM loop_config WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_wake_events WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_wake_state WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_revisions WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_snapshots WHERE workspace_id = OLD.id;
	DELETE FROM agent_soul_revisions WHERE workspace_id = OLD.id;
	DELETE FROM agent_soul_snapshots WHERE workspace_id = OLD.id;
	DELETE FROM session_health WHERE workspace_id = OLD.id;
	DELETE FROM sessions WHERE workspace_id = OLD.id;
	DELETE FROM token_usage_daily WHERE workspace_id = OLD.id;
	DELETE FROM event_summaries WHERE workspace_id = OLD.id;
	DELETE FROM tool_approval_grants WHERE workspace_id = OLD.id;
	DELETE FROM dead_entities WHERE workspace_id = OLD.id;
	DELETE FROM notification_cursors WHERE workspace_id = OLD.id;
	DELETE FROM skill_exposures WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd

PRAGMA foreign_keys = on;
