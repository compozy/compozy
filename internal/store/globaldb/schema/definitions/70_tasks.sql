CREATE TABLE scheduler_pause (
			id         INTEGER PRIMARY KEY CHECK (id = 1),
			paused     INTEGER NOT NULL DEFAULT 0 CHECK (paused IN (0, 1)),
			paused_by  TEXT NOT NULL DEFAULT '',
			paused_at  TEXT,
			reason     TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

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

CREATE INDEX idx_task_events_wake_event
ON task_events(task_id, event_type, json_extract(payload_json, '$.wake_event_id'))
WHERE event_type IN ('task.wake.delivered', 'task.wake.suppressed');

CREATE INDEX idx_task_triage_actor ON task_triage_state(actor_kind, actor_id, updated_at DESC, task_id);

CREATE INDEX idx_task_triage_task ON task_triage_state(task_id, updated_at DESC);

CREATE INDEX idx_tasks_approval_state ON tasks(approval_state);

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

CREATE UNIQUE INDEX uq_task_events_event_seq ON task_events(event_seq);
