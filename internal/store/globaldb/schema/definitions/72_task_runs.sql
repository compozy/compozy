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
		task_id         TEXT REFERENCES tasks(id) ON DELETE CASCADE,
		workspace_id    TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		status          TEXT NOT NULL,
		attempt         INTEGER NOT NULL CHECK (attempt > 0),
		recovery_count  INTEGER NOT NULL DEFAULT 0 CHECK (recovery_count >= 0),
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
		network_spec_json TEXT NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}'
			CHECK (json_valid(network_spec_json)),
		network_mode TEXT NOT NULL DEFAULT 'local' CHECK (network_mode IN ('local', 'live')),
		network_channel TEXT,
		network_source TEXT NOT NULL DEFAULT 'built_in_local' CHECK (
			network_source IN (
				'explicit_request', 'task_profile', 'workspace_coordination',
				'loop_definition', 'automation_job', 'built_in_local'
			)
		),
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
		run_kind TEXT NOT NULL DEFAULT 'worker' CHECK (run_kind IN ('worker', 'coordinator', 'network_wake')),
		loop_run_id TEXT,
		tokens_used INTEGER NOT NULL DEFAULT 0 CHECK (tokens_used >= 0),
		network_wake_id TEXT,
		network_target_session_id TEXT,
		network_owner_key TEXT,
		CHECK (
			(claimed_by_kind IS NULL AND claimed_by_ref IS NULL) OR
			(claimed_by_kind IS NOT NULL AND claimed_by_ref IS NOT NULL)
		),
		CHECK (status <> 'queued' OR session_id IS NULL),
		CHECK (run_kind = 'network_wake' OR task_id IS NOT NULL),
		CHECK (run_kind <> 'network_wake' OR task_id IS NULL),
		CHECK (run_kind <> 'network_wake' OR workspace_id IS NOT NULL),
		CHECK (
			(run_kind = 'network_wake' AND network_wake_id IS NOT NULL
				AND network_target_session_id IS NOT NULL AND network_owner_key IS NOT NULL) OR
			(run_kind <> 'network_wake' AND network_wake_id IS NULL
				AND network_target_session_id IS NULL AND network_owner_key IS NULL)
		),
		FOREIGN KEY (workspace_id, network_target_session_id)
			REFERENCES sessions(workspace_id, id) ON DELETE CASCADE,
		UNIQUE (workspace_id, id)
	);

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

CREATE INDEX idx_task_runs_target_session
			ON task_runs(network_target_session_id, status, queued_at, id)
			WHERE run_kind = 'network_wake';

CREATE INDEX idx_task_runs_status ON task_runs(status);

CREATE INDEX idx_task_runs_task ON task_runs(task_id, queued_at DESC, id DESC);

CREATE INDEX idx_task_runs_task_review_round
		ON task_runs(task_id, review_round)
		WHERE review_round > 0;

CREATE INDEX idx_task_runs_task_status ON task_runs(task_id, status, queued_at DESC, id DESC);

CREATE INDEX idx_task_runs_workspace_active
			ON task_runs(workspace_id, status, lease_until)
			WHERE workspace_id IS NOT NULL AND run_kind IN ('worker', 'coordinator');

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
