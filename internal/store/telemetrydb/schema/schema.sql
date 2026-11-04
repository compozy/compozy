CREATE TABLE telemetry_identity (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    identity TEXT NOT NULL CHECK (length(identity) > 0)
);

CREATE TABLE event_summaries (
            seq INTEGER PRIMARY KEY AUTOINCREMENT,
			id                     TEXT NOT NULL UNIQUE,
			profile_id             TEXT NOT NULL,
			session_id             TEXT NOT NULL DEFAULT '',
			workspace_id           TEXT NOT NULL DEFAULT '',
			worktree_id            TEXT NOT NULL DEFAULT '',
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

CREATE INDEX idx_summaries_actor ON event_summaries(actor_kind, actor_id);


CREATE INDEX idx_summaries_failures_timestamp ON event_summaries(timestamp DESC) WHERE outcome IN ('failure', 'warning');


CREATE INDEX idx_summaries_provider_timestamp ON event_summaries(provider, timestamp DESC);


CREATE INDEX idx_summaries_run ON event_summaries(run_id);

CREATE INDEX idx_summaries_session ON event_summaries(session_id);

CREATE INDEX idx_summaries_task ON event_summaries(task_id);

CREATE INDEX idx_summaries_timestamp ON event_summaries(timestamp);

CREATE INDEX idx_summaries_type ON event_summaries(type);


CREATE INDEX idx_summaries_workspace ON event_summaries(workspace_id);

CREATE INDEX idx_summaries_worktree ON event_summaries(worktree_id);

CREATE INDEX idx_summaries_profile_timestamp ON event_summaries(profile_id, timestamp DESC);
CREATE INDEX idx_summaries_profile_sequence ON event_summaries(profile_id, seq);
