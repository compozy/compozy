CREATE TABLE event_summaries (
			id                     TEXT PRIMARY KEY,
			session_id             TEXT NOT NULL DEFAULT '',
			workspace_id           TEXT NOT NULL DEFAULT '',
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

CREATE INDEX idx_summaries_hook_event ON event_summaries(hook_event);

CREATE INDEX idx_summaries_outcome_timestamp ON event_summaries(outcome, timestamp DESC);

CREATE INDEX idx_summaries_parent ON event_summaries(parent_session_id);

CREATE INDEX idx_summaries_provider_timestamp ON event_summaries(provider, timestamp DESC);

CREATE INDEX idx_summaries_root ON event_summaries(root_session_id);

CREATE INDEX idx_summaries_run ON event_summaries(run_id);

CREATE INDEX idx_summaries_session ON event_summaries(session_id);

CREATE INDEX idx_summaries_task ON event_summaries(task_id);

CREATE INDEX idx_summaries_timestamp ON event_summaries(timestamp);

CREATE INDEX idx_summaries_type ON event_summaries(type);

CREATE INDEX idx_summaries_workflow ON event_summaries(workflow_id);

CREATE INDEX idx_summaries_workspace ON event_summaries(workspace_id);
